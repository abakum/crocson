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

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	log "github.com/schollz/logger"
)

// Built-in QR scanner (Camera1, Android). The camera preview is rendered in a
// native Android Dialog (TextureView color surface), NOT in Fyne — that gives
// the camera a real, consumed surface (no dummy-texture stall on old HALs) and
// keeps the heavy rendering off the Go/GL thread. Java still feeds the NV21 Y
// plane here via the JNI bridge for QR decode with gozxing (ZXing port, has
// perspective correction). Decode is the only Go per-frame work.

var (
	qrMu        sync.Mutex
	qrActive    bool
	qrStop      atomic.Bool
	qrDecodeCh  chan *image.Gray
	qrCancel    context.CancelFunc
	qrReader    gozxing.Reader
	qrOnResult  func(string)
	qrRecvCount atomic.Int64
	// qrCameraStarted is true while the native camera Dialog is up. Guards
	// qrLifecyclePause: only dismiss on a pause that happens while actually
	// scanning (the first-run permission-request pause happens before any camera
	// Dialog is shown -> qrCameraStarted still false -> no-op).
	qrCameraStarted bool
	// qrRot is the number of 90 deg CW rotations to apply to the Y plane for QR
	// DECODE so gozxing sees an upright code. Recomputed (throttled) in
	// qrDecodeWorker from qrSensorOrient and the live device rotation. The visible
	// preview is rotated natively by setDisplayOrientation; only the raw
	// onPreviewFrame bytes (used for decode) arrive in sensor-native orientation.
	qrRot          int
	qrSensorOrient int
)

// cameraFrameReceived is called from the camera thread (via cgo export
// cameraFrameNotify in for_android.go) for each NV21 preview frame, already
// pre-cropped to a centered square Y by Java.
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
	// data is already a fresh owned slice from C.GoBytes; the Y plane is its
	// first w*h bytes, so wrap it directly (no extra make+copy). The decoder is
	// the only consumer and reads it read-only.
	g := &image.Gray{Pix: data[:n], Stride: w, Rect: image.Rect(0, 0, w, h)}
	select {
	case qrDecodeCh <- g:
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

// qrDecodeWorker consumes prepared Y frames and runs gozxing.Decode, which is
// the only heavy per-frame work. The visible preview is rendered natively, so
// decode runs independently of the preview rate (a slow decode never freezes
// the preview). On a successful decode it stops the camera and routes the text.
func qrDecodeWorker(ctx context.Context) {
	var lastRot time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case g := <-qrDecodeCh:
			if qrStop.Load() {
				continue
			}
			y := g.Pix
			w, h := g.Rect.Dx(), g.Rect.Dy()
			if w != h {
				// Fallback: frame not pre-cropped to a square by Java.
				y, w, h = cropCenterSquare(y, w, h)
			}
			// Device orientation changes slowly (human grip), so throttle the
			// JNI query (~4x/sec) instead of once per frame. First frame always
			// queries (lastRot is zero). Sticky on error.
			if time.Since(lastRot) >= 250*time.Millisecond {
				if devRot, err := callInt("getDeviceRotation"); err == nil && devRot >= 0 {
					qrRot = (((qrSensorOrient - devRot) % 360) + 360) % 360 / 90
				}
				lastRot = time.Now()
			}
			for i := 0; i < qrRot; i++ {
				y, w, h = rotateGray90cw(y, w, h)
			}
			dec := &image.Gray{Pix: y, Stride: w, Rect: image.Rect(0, 0, w, h)}
			bmp, err := gozxing.NewBinaryBitmapFromImage(dec)
			if err != nil {
				continue
			}
			res, derr := qrReader.Decode(bmp, nil)
			if derr == nil && res != nil {
				text := strings.TrimSpace(res.GetText())
				log.Debugf("decoded %s", text)
				qrStop.Store(true)
				qrFinish()
				callVoid("dismissCameraDialog")
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
	if cb != nil {
		fyne.Do(func() { cb(text) })
	}
}

func hasCameraPermission() bool {
	ok, _ := callBoolean("hasCameraPermission")
	return ok
}

func requestCameraPermission() {
	callBoolean("requestCameraPermission")
}

// startQRScan opens the native camera Dialog and scans until a QR is found or
// the user cancels. onResult (may be nil) is invoked on the Fyne thread with
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
	qrDecodeCh = make(chan *image.Gray, 1)
	qrStop.Store(false)
	qrRecvCount.Store(0)
	qrRot = 0           // recomputed (throttled) in qrDecodeWorker
	qrSensorOrient = 90 // safe default for back cameras
	if o, err := callInt("getCameraSensorOrientation"); err == nil && o >= 0 {
		qrSensorOrient = o
		log.Debugf("getCameraSensorOrientation %d", o)
	}

	var ctx context.Context
	ctx, qrCancel = context.WithCancel(context.Background())
	go qrDecodeWorker(ctx)

	if hasCameraPermission() {
		qrShowCamera()
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
				qrShowCamera()
				return
			}
		}
		log.Debug("camera permission timed out")
		callVoidString("showToast", lp("Camera permission denied"))
		qrFinish()
	}()
}

func qrStillActive() bool {
	qrMu.Lock()
	defer qrMu.Unlock()
	return qrActive
}

// qrShowCamera opens the native camera Dialog (Java) and marks the scan as
// running. The Dialog shows immediately; the camera opens asynchronously once
// the TextureView surface is ready (a later "cameraOpenFailed" lifecycle event
// handles open failure). Caller must have CAMERA permission already granted.
func qrShowCamera() {
	if !qrStillActive() {
		return
	}
	if err := callVoid("showCameraDialog"); err != nil {
		log.Debugf("showCameraDialog failed: %v", err)
		callVoidString("showToast", lp("Camera unavailable"))
		qrFinish()
		return
	}
	qrMu.Lock()
	qrCameraStarted = true
	qrMu.Unlock()
}

// qrFinish tears down the Go scan state (goroutines, flags, callback). It does
// NOT touch the native camera Dialog — callers decide whether to dismiss it via
// dismissCameraDialog. Idempotent.
func qrFinish() {
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
	qrOnResult = nil
}

// qrLifecyclePause is invoked from the lifecycle goroutine on activity "pause".
// If a scan is active and the camera Dialog is up, it dismisses the Dialog
// (which stops the camera) and tears down the Go state. The qrCameraStarted
// guard excludes the first-run permission request: its system dialog also
// triggers onPause, but the camera Dialog is never shown before permission is
// granted (qrCameraStarted still false). No resume: the scan is abandoned; the
// user re-opens it to scan again.
func qrLifecyclePause() {
	qrMu.Lock()
	closeDialog := qrActive && qrCameraStarted
	qrMu.Unlock()
	if closeDialog {
		callVoid("dismissCameraDialog")
		qrFinish()
	}
}

// qrLifecycleCancel is invoked on the "qrCancel" lifecycle event (user tapped
// Cancel or pressed Back). Java has already dismissed the Dialog + stopped the
// camera; here we only tear down the Go state.
func qrLifecycleCancel() {
	qrFinish()
}

// qrCameraOpenFailed is invoked on the "cameraOpenFailed" lifecycle event: Java
// could not open/start the camera. The Dialog is already dismissed by Java.
func qrCameraOpenFailed() {
	callVoidString("showToast", lp("Camera unavailable"))
	qrFinish()
}

// stopQRScan dismisses the native camera Dialog and tears down the decode loop.
// Idempotent.
func stopQRScan() {
	qrFinish()
	callVoid("dismissCameraDialog")
}
