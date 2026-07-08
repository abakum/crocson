//go:build android

package main

import (
	"context"
	"fmt"
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
	// frame so the viewfinder matches the device orientation. Derived from the
	// camera sensor orientation (see getCameraSensorOrientation): Camera1 delivers
	// NV21 frames in the sensor's native landscape orientation regardless of
	// setDisplayOrientation, so we rotate the Y plane ourselves.
	qrRot int
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
		LogD(fmt.Sprintf("qr: frame %dx%d #%d", w, h, c))
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

func qrDecodeLoop(ctx context.Context) {
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
			for i := 0; i < qrRot; i++ {
				y, w, h = rotateGray90cw(y, w, h)
			}
			gray := &image.Gray{Pix: y, Stride: w, Rect: image.Rect(0, 0, w, h)}

			g := gray
			fyne.Do(func() {
				qrPreview.Image = g
				qrPreview.Refresh()
			})

			bmp, err := gozxing.NewBinaryBitmapFromImage(gray)
			if err != nil {
				continue
			}
			res, derr := qrReader.Decode(bmp, nil)
			if derr == nil && res != nil {
				text := strings.TrimSpace(res.GetText())
				LogD("qr: decoded " + text)
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
			LogD("qr: uriFromIntent full")
		}
	} else {
		select {
		case textFromIntent <- text:
		default:
			LogD("qr: textFromIntent full")
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
	qrStop.Store(false)
	qrRecvCount.Store(0)
	qrRot = 0
	if o, err := callInt("getCameraSensorOrientation"); err == nil && o >= 0 {
		// The NV21 frame arrives in the sensor's native orientation; rotating it
		// (orientation/90) quarters CW makes it upright in portrait.
		qrRot = (o / 90) % 4
	}

	var ctx context.Context
	ctx, qrCancel = context.WithCancel(context.Background())
	go qrDecodeLoop(ctx)

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
		LogD("qr: camera permission timed out")
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
		LogD("qr: startCamera failed: " + fmt.Sprint(err))
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
		LogD("qr: stopCamera: " + fmt.Sprint(err))
	}
	qrHideDialog()
	qrOnResult = nil
}
