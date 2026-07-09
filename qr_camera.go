//go:build android

package main

import (
	"context"
	"image"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	log "github.com/schollz/logger"
)

// Built-in QR scanner (Camera1, Android). Java feeds NV21 preview frames here
// via the JNI bridge; the Y(luma) plane is decoded with gozxing (ZXing port,
// has perspective correction). The viewfinder is rendered in Fyne.

type cameraFrame struct {
	y    []byte
	w, h int
}

var (
	qrMu        sync.Mutex
	qrActive    bool
	qrStop      atomic.Bool
	qrFrameCh   chan cameraFrame
	qrDecodeCh  chan *image.Gray
	qrCancel    context.CancelFunc
	qrReader    gozxing.Reader
	qrPreview   *canvas.Image
	qrStatus    *widget.Label
	qrDialog    dialog.Dialog
	qrOnResult  func(string)
	qrRecvCount atomic.Int64
	// qrCameraStarted is true while the camera hardware is running. Used by
	// qrLifecyclePause to decide whether a pause (activity change) should close
	// the scan dialog: the camera only runs after permission is granted, so the
	// first-run permission-request pause (qrCameraStarted still false) is skipped.
	qrCameraStarted bool
	// qrRot is the number of 90° clockwise rotations to apply to each preview
	// frame so the viewfinder matches the current device orientation. Recomputed
	// per decoded frame from qrSensorOrient and the live device rotation
	// (getDeviceRotation): the NV21 Y plane always arrives in the sensor's native
	// landscape orientation, and setDisplayOrientation only affects a surface
	// (we have none), so we rotate the Y plane ourselves.
	qrRot int
	// qrSensorOrient is the back camera's fixed sensor orientation in degrees
	// (info.orientation, ~90 on virtually all back cameras). Cached once at scan
	// start; combined with the live device rotation to derive qrRot.
	qrSensorOrient int
)

// cameraFrameReceive is called from the camera thread (via cgo export
// cameraFrameNotify in for_android.go) for each NV21 preview frame.
// Returns true to keep streaming, false to tell Java to stop feeding frames.
func cameraFrameReceived(data []byte, w, h int) bool {
	if qrStop.Load() {
		return false
	}
	n := w * h
	if n <= 0 || len(data) < n {
		return true // ignore bad frame, keep camera going
	}
	c := qrRecvCount.Add(1)
	if c == 1 || c%30 == 0 {
		log.Debugf("frame %dx%d #%d", w, h, c)
	}
	// Keep only the Y(luma) plane: enough for both preview and QR decode.
	y := make([]byte, n)
	copy(y, data[:n])
	select {
	case qrFrameCh <- cameraFrame{y: y, w: w, h: h}:
	default:
		// decoder busy (drop-latest): back-pressure, but keep the camera streaming
	}
	return true
}

// rotateGray90cw rotates a grayscale buffer (w wide, h tall, stride w) 90°
// clockwise, returning a new buffer of size h wide × w tall.
func rotateGray90cw(src []byte, w, h int) (dst []byte, nw, nh int) {
	nw, nh = h, w
	dst = make([]byte, w*h)
	for r := 0; r < w; r++ { // new row
		for c := 0; c < h; c++ { // new col
			dst[r*nw+c] = src[(h-1-c)*w+r]
		}
	}
	return dst, nw, nh
}

func cropCenterSquare(src []byte, w, h int) (dst []byte, nw, nh int) {
	nw = w
	nh = h
	side := w
	if h < w {
		side = h
	}
	if w == h {
		return src, w, h
	}
	xoff := (w - side) / 2
	yoff := (h - side) / 2
	dst = make([]byte, side*side)
	for r := 0; r < side; r++ {
		rowSrc := src[(yoff+r)*w+xoff : (yoff+r)*w+xoff+side]
		copy(dst[r*side:r*side+side], rowSrc)
	}
	nw = side
	nh = side
	return dst, nw, nh
}

// qrPreviewLoop drains camera frames at the camera rate: crop, rotate to the
// current device orientation, and refresh the viewfinder. Decoding (the heavy
// part, hundreds of ms on slow devices) is NOT done here — each prepared frame
// is handed to qrDecodeWorker via qrDecodeCh (drop-latest, non-blocking). This
// keeps the viewfinder smooth (full camera rate) regardless of how slow the
// per-frame QR decode is (e.g. ~2 fps decode on Android 9 still gives a smooth
// preview, instead of freezing at the decode rate).
func qrPreviewLoop(ctx context.Context) {
	var lastRot time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case fr := <-qrFrameCh:
			if qrStop.Load() {
				continue
			}
			y := fr.y
			w, h := fr.w, fr.h
			if w != h {
				// Fallback: frame not pre-cropped to a square by Java.
				y, w, h = cropCenterSquare(y, w, h)
			}
			// Device orientation changes slowly (human grip), so throttle the
			// JNI query (~4x/sec) instead of once per frame. The first frame
			// always queries (lastRot is zero). Sticky on error.
			if time.Since(lastRot) >= 250*time.Millisecond {
				if devRot, err := callInt("getDeviceRotation"); err == nil && devRot >= 0 {
					qrRot = (((qrSensorOrient - devRot) % 360) + 360) % 360 / 90
				}
				lastRot = time.Now()
			}
			for i := 0; i < qrRot; i++ {
				y, w, h = rotateGray90cw(y, w, h)
			}
			gray := &image.Gray{Pix: y, Stride: w, Rect: image.Rect(0, 0, w, h)}

			g := gray
			fyne.Do(func() {
				qrPreview.Image = g
				qrPreview.Refresh()
			})

			// Hand the prepared frame to the decoder. Drop-latest: if the
			// previous decode is still running, skip — no backlog, no stale
			// queue. The decoder reads gray read-only, so this is safe to share
			// with the Fyne render thread above.
			select {
			case qrDecodeCh <- g:
			default:
			}
		}
	}
}

// qrDecodeWorker runs gozxing.Decode on the latest prepared frame, independently
// of the preview rate. A successful decode stops the camera and routes the text.
func qrDecodeWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case g := <-qrDecodeCh:
			if qrStop.Load() {
				continue
			}
			bmp, err := gozxing.NewBinaryBitmapFromImage(g)
			if err != nil {
				continue
			}
			res, derr := qrReader.Decode(bmp, nil)
			if derr == nil && res != nil {
				text := strings.TrimSpace(res.GetText())
				log.Debugf("decoded %s", text)
				qrStop.Store(true)
				callVoid("stopCamera")
				qrRoute(text)
				return
			}
		}
	}
}

// qrRoute delivers the decoded text through the existing intent channels so the
// send-tab handler picks it up (textFromIntent for codes, uriFromIntent for IO-links).
func qrRoute(text string) {
	if text == "" {
		return
	}
	if strings.HasPrefix(text, IO) {
		select {
		case uriFromIntent <- text:
		default:
			log.Debug("uriFromIntent full")
		}
	} else {
		select {
		case textFromIntent <- text:
		default:
			log.Debug("textFromIntent full")
		}
	}
	cb := qrOnResult
	fyne.Do(func() {
		qrHideDialog()
		if cb != nil {
			cb(text)
		}
	})
}

func hasCameraPermission() bool {
	ok, _ := callBoolean("hasCameraPermission")
	return ok
}

func requestCameraPermission() {
	callBoolean("requestCameraPermission")
}

// startQRScan opens the in-app camera viewfinder and scans until a QR is found
// or the user cancels. onResult (may be nil) is invoked on the Fyne thread with
// the decoded text.
func startQRScan(a fyne.App, w fyne.Window, onResult func(string)) {
	qrMu.Lock()
	if qrActive {
		qrMu.Unlock()
		return
	}
	qrActive = true
	qrMu.Unlock()

	qrOnResult = onResult
	qrReader = qrcode.NewQRCodeReader()
	qrFrameCh = make(chan cameraFrame, 1)
	qrDecodeCh = make(chan *image.Gray, 1)
	qrStop.Store(false)
	qrRecvCount.Store(0)
	qrRot = 0           // recomputed (throttled) in qrPreviewLoop
	qrSensorOrient = 90 // safe default for back cameras
	if o, err := callInt("getCameraSensorOrientation"); err == nil && o >= 0 {
		qrSensorOrient = o
		log.Debugf("getCameraSensorOrientation %d", o)
	}

	var ctx context.Context
	ctx, qrCancel = context.WithCancel(context.Background())
	go qrPreviewLoop(ctx)
	go qrDecodeWorker(ctx)

	// Fully-opaque solid black placeholder (every pixel black, including alpha),
	// so the pre-camera state is a clean black square rather than a misleading
	// transparent-pixel gradient.
	black := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := 0; i < len(black.Pix); i += 4 {
		black.Pix[i+3] = 255 // alpha = opaque (RGB already 0,0,0)
	}
	qrPreview = canvas.NewImageFromImage(black)
	qrPreview.FillMode = canvas.ImageFillContain
	qrPreview.SetMinSize(fyne.NewSize(qrSize, qrSize))
	qrStatus = widget.NewLabel(lp(builtinScanner))
	qrStatus.Alignment = fyne.TextAlignCenter

	qrShowDialog(w)

	if hasCameraPermission() {
		qrBegin(w)
		return
	}

	// No permission yet: ask and poll until granted (or timeout).
	requestCameraPermission()
	go func() {
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(250 * time.Millisecond)
			if !qrStillActive() {
				return // cancelled
			}
			if hasCameraPermission() {
				fyne.Do(func() { qrBegin(w) })
				return
			}
		}
		log.Debug("camera permission timed out")
		fyne.Do(func() {
			if qrStatus != nil {
				qrStatus.SetText(lp("Camera permission denied"))
			}
		})
	}()
}

func qrStillActive() bool {
	qrMu.Lock()
	defer qrMu.Unlock()
	return qrActive
}

// qrBegin starts the camera; on failure shows a message in the viewfinder.
func qrBegin(w fyne.Window) {
	if !qrStillActive() {
		return
	}
	ok, err := callBoolean("startCamera")
	if err != nil || !ok {
		log.Debugf("startCamera failed: %v", err)
		qrStatus.SetText(lp("Camera unavailable"))
		return
	}
	qrMu.Lock()
	qrCameraStarted = true
	qrMu.Unlock()
}

// qrLifecyclePause is invoked from the lifecycle goroutine on activity "pause".
// If a scan is active and the camera is running, it closes the scan dialog
// (which stops the camera). The qrCameraStarted guard excludes the first-run
// permission request: its system dialog also triggers onPause, but the camera
// is never started before permission is granted. stopQRScan touches UI, so it
// runs on the Fyne thread via fyne.Do.
func qrLifecyclePause() {
	qrMu.Lock()
	closeDialog := qrActive && qrCameraStarted
	qrMu.Unlock()
	if closeDialog {
		fyne.Do(func() { stopQRScan() })
	}
}

func qrShowDialog(w fyne.Window) {
	content := container.NewVBox(
		container.NewCenter(qrPreview),
		widget.NewSeparator(),
		qrStatus,
	)
	d := dialog.NewCustom(lp("Scan QRs"), lp("Cancel"), content, w)
	d.SetOnClosed(func() { stopQRScan() })
	qrDialog = d
	d.Show()
}

func qrHideDialog() {
	if qrDialog != nil {
		d := qrDialog
		qrDialog = nil
		d.Hide()
	}
}

// stopQRScan releases the camera and tears down the decode loop. Idempotent.
func stopQRScan() {
	qrMu.Lock()
	active := qrActive
	qrActive = false
	qrCameraStarted = false
	qrMu.Unlock()
	if !active {
		return
	}
	qrStop.Store(true)
	if qrCancel != nil {
		qrCancel()
	}
	if err := callVoid("stopCamera"); err != nil {
		log.Debugf("stopCamera: %v", err)
	}
	qrHideDialog()
	qrOnResult = nil
}
