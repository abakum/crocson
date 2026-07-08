# Built-in QR camera scanner (Android) via Java + Go + gozxing

## Status: PLANNED — ready for implementation

Add an **in-app** QR scanner to crocson so a croc code / `IO`-link can be read by pointing the device camera, without launching external scanner intents or the html5-qrcode web page. Decode is done in Go with `makiuchi-d/gozxing` (ZXing port, has perspective correction). Camera capture is done in Java (framework API) and frames are handed to Go over the existing JNI bridge.

## Key decisions (all confirmed)
- **Source**: live camera, Android only. Desktop / non-Android keeps the current fallback (external intent scanners / browser `https://abakum.github.io/scan/`).
- **Decoder**: `github.com/makiuchi-d/gozxing` (NOT `piglig/go-qr` — it lacks perspective correction; on angled camera shots gozxing is far more reliable).
- **Capture API**: Camera1 `android.hardware.Camera` (framework, deprecated but works on all devices, hands NV21 frames directly via `setPreviewCallbackWithBuffer`). Camera2 is the only realistic alternative; CameraX is **ruled out** (see "AndroidX is impossible" below).
- **Viewfinder**: rendered inside Fyne (`canvas.Image`) from the Y(luma) plane of each NV21 frame → grayscale preview, ~8 fps. No native `SurfaceView` overlay (avoids GL-surface z-order conflicts; matches "everything in Fyne" style).
- **Result routing**: decoded text → existing `textFromIntent` channel (handler `send.go:1637`); if `strings.HasPrefix(text, IO)` → `uriFromIntent` (browser-scan path already keys on the `IO` prefix, `send.go:1620-1625`).
- **Bridge**: control (start/stop/permission) reuses existing generic `callVoid`/`callBoolean` (`for_android.c:15,81`) — no new C for control. The only new C is the Java→Go byte[] frame path, modelled on `intentText` (`for_android.c:470`).

## AndroidX / CameraX is impossible (verified, code-level)
The installed `fyne` CLI is `fyne.io/tools` @ `42ed49e0…` (`go version -m /home/koka/go/bin/fyne`). It compiles project `.java` itself:
- activity: `cmd/fyne/internal/mobile/compile_activity.go:58-65`
  `javac -source 1.8 -sourcepath <dir> -bootclasspath <platform>/android.jar -d … GoNativeActivity.java` — **no `-classpath`**, only `android.jar`.
- services: `cmd/fyne/internal/mobile/build_androidapp.go:538-543` — same, `-bootclasspath android.jar`, no classpath.
- `d8` dexes **only** the project's own `.class` files (`compile_activity.go:92-93`, `build_androidapp.go:571-572`). Library classes are never dexed.

Two independent blockers:
1. `import androidx.camera.*` fails to compile — `androidx.*` is not in `android.jar`.
2. Even if compiled, CameraX classes wouldn't be in the APK → runtime `ClassNotFoundException`.

Conclusion: only `android.jar` framework classes are usable (`android.hardware.Camera`, `android.hardware.camera2.*`, `ImageReader`, `YuvImage`, `SurfaceTexture`, etc.). To enable CameraX you'd have to fork `fyne` (add `-classpath` + feed library classes to `d8` + pull transitive androidx deps) — out of scope.

## Architecture / data flow
```
[Fyne "Встроенный" button]
   │ Go: ensure CAMERA permission (callBoolean "hasCameraPermission"; else callBoolean "requestCameraPermission")
   │ Go: callBoolean "startCamera"  → false: show error / fall back to external scanners
   ▼
[Java GoNativeActivity: Camera.open(back); setPreviewFormat(NV21); setPreviewSize; addCallbackBuffer]
   │ per frame: native boolean cameraFrame(byte[] data, int w, int h)
   │   → true: accept, re-addCallbackBuffer(buf)  |  false: stop preview (release buffer)
   ▼
[C bridge: jboolean Java_..._GoNativeActivity_cameraFrame (GetByteArrayElements → copy) ]
   │ //export cameraFrameNotify(data []byte, w, h int) bool
   ▼
[Go qr_camera.go: drop-latest chan(1); build *image.Gray from Y plane (first w*h bytes)]
   ├─ gozxing.NewBinaryBitmapFromImage(gray) → Decode → on hit: stop, route text
   └─ fyne.Do: refresh canvas.Image (grayscale viewfinder) + status label
```
Stop conditions: decode success / user Cancel / lifecycle `pause`|`stop` (Go already listens on `lifecycleFromJava`, `send.go:1617`) / Java `onPause` safety guard.

## Native callback contract (`cameraFrame` returns `boolean`)
Mirrors the existing "helpers return `false` on error" idiom (`callBoolean`/`callBooleanString` → jint 0/1, `for_android.c:81,133`):
- `cameraFrameNotify` returns **`true`** → frame accepted, scanning continues; Java re-`addCallbackBuffer(buf)`.
- returns **`false`** → Go requests "stop feeding frames": decode succeeded, user Cancel, paused, or scanner closed; Java stops preview (idempotent — safe to also call `stopCamera()`).
- Back-pressure is NOT a stop: if the 1-slot drop-latest channel is full, Go drops the frame internally and **still returns `true`** (don't stall the camera for a transiently-busy decoder).
- C-side mapping: `JNIEXPORT jboolean Java_..._cameraFrame` calls the cgo export and returns `r ? JNI_TRUE : JNI_FALSE`; on any JNI error (`caseException`) return `JNI_FALSE` so Java stops rather than spinning.

## Ordered task list

### 1. Go module
- `go get github.com/makiuchi-d/gozxing`; tidy. Verify the decode symbol is `gozxing.NewBinaryBitmapFromImage(image.Image) (*BinaryBitmap, error)` then `bmp.Decode(nil)` → `result.GetText()` (adjust to actual API).

### 2. Manifest — `AndroidManifest.xml`
- Add `<uses-permission android:name="android.permission.CAMERA"/>`.
- Add `<uses-feature android:name="android.hardware.camera" android:required="false"/>` (install allowed on camera-less devices).

### 3. Java — `GoNativeActivity.java` (new `static` helpers + native decl + permission handling)
- Native decl (near `GoNativeActivity.java:59`): `private native boolean cameraFrame(byte[] data, int w, int h);` (returns `false` to signal stop/error — see "Native callback contract").
- `static boolean hasCameraPermission()` → `checkSelfPermission(CAMERA)==GRANTED`.
- `static boolean requestCameraPermission()` → `try { requestPermissions(new String[]{CAMERA}, 201); return true; } catch (Throwable t) { Log.e(...); return false; }` (requestCode **201**, 123 is taken by storage). Wraps in try/catch like `acquireMulticastLock` (`GoNativeActivity.java:199`); on grant/deny emit a lifecycle event so Go reacts.
- `static boolean startCamera()` / `static void stopCamera()` — `startCamera` returns **`false`** on any failure (the one that actually throws: `Camera.open`, `setParameters`), `true` once streaming:
  - open **back** camera (`Camera.open(findBack())`); keep a static `Camera cam`. Wrap open+configure in try/catch → on failure release what's open and `return false`.
  - Parameters: `setPreviewFormat(ImageFormat.NV21)`, small `setPreviewSize` (e.g. 640×480), `setFocusMode(FOCUS_MODE_CONTINUOUS_PICTURE)` if supported.
  - `setPreviewCallbackWithBuffer((data, cam) -> { if (!cameraFrame(data, w, h)) stopCamera(); });` then `addCallbackBuffer(buf)` to prime; on `true` re-`addCallbackBuffer` after each emit (on `false` stop, as above).
  - Apply display orientation from `CameraInfo.orientation` (note preview frames are still rotated; Go uses luma only — see task 5).
  - `stopCamera()`: best-effort, try/catch-wrapped (teardown must never throw) — `setPreviewCallbackWithBuffer(null)`, `stopPreview()`, `release()`, `cam=null`.
- Permission result: extend `onRequestPermissionsResult` (`:1048`, currently handles 123) for code 201 → emit lifecycle event (e.g. `"cameraPermission"`) so Go resumes or shows denial.
- Safety: in `onPause` call the equivalent of stopCamera (or rely on the Go lifecycle listener — pick one; document which).

### 4. C bridge — `for_android.c` + `for_android.h`
- Add `jboolean Java_org_golang_app_GoNativeActivity_cameraFrame(JNIEnv* env, jobject thiz, jbyteArray data, jint w, jint h)` in `for_android.c` (modelled on `intentText` at `:470` + the boolean idiom of `callBoolean` at `:81`): `GetByteArrayElements` → copy into a Go-bound slice, call the cgo export, `ReleaseByteArrayElements`; return its `bool` as `JNI_TRUE`/`JNI_FALSE`; on `caseException` return `JNI_FALSE`. Declare in `for_android.h`.
- The cgo export lives in Go (task 5): `//export cameraFrameNotify`.
- Control calls need **no** new C — reuse the generic bridges: `callBoolean` for `hasCameraPermission`/`requestCameraPermission`/`startCamera` (each returns `false` on failure → Go shows error / falls back), `callVoid` for `stopCamera` (best-effort teardown).

### 5. Go (android, `//go:build android`) — new `qr_camera.go` (+ add the `//export`)
- `//export cameraFrameNotify(data []byte, w, h int) bool` — returns whether Java should keep streaming. Feed a 1-slot drop-latest channel: if full, drop the frame but return `true` (back-pressure ≠ stop). Return `false` once a `stopRequested` flag is set (decode hit / cancel / paused / closed) so Java stops promptly without a separate control round-trip.
- Decoder goroutine: `y := data[:w*h]`; build `*image.Gray` (Pix=y, Stride=w, Rect w×h); `bmp,_ := gozxing.NewBinaryBitmapFromImage(gray); res,err := bmp.Decode(nil)`. On success: set `stopRequested`, `stopCamera()`; route result:
  - `if strings.HasPrefix(text, IO) { uriFromIntent <- text } else { textFromIntent <- text }`
  - then `fyne.Do(...)` close scan dialog.
- Preview: same `*image.Gray` → update a `*canvas.Image` resource via `fyne.Do` + status label.
- Public funcs: `startQRScan(a, w, onResult func(string))` / `stopQRScan()` that wrap the permission + lifecycle flow and open/close a Fyne `Dialog` with the viewfinder + Cancel button:
  - `if !callBoolean("hasCameraPermission") { if !callBoolean("requestCameraPermission") { <fallback/error>; return } ; /* await grant via lifecycle event, then retry */ }`
  - `if !callBoolean("startCamera") { <show "camera unavailable", fall back to external scanners>; return }`
  - `stopQRScan()` → `callVoid("stopCamera")` + set `stopRequested`.
- Lifecycle: on `lifecycleFromJava` "pause"/"stop", call `stopQRScan()`.
- Orientation note: gozxing is rotation-tolerant; if detection is poor on portrait shots, optionally rotate the gray buffer by 90° using `CameraInfo.orientation` before decode (polish — keep optional).

### 6. Go (non-android, `for_android0.go`) — stubs
- `func startQRScan(...) {}` / `func stopQRScan() {}` no-ops (or return "not supported" so the UI hides the option).

### 7. UI — `applinks.go`
- `makeScannerSettings` (`:335`): add option `"Встроенный (камера)"` to `optScan` **only when `isAndroid || asMobile`**.
- `scanner()` (`:433`): if selected option is the built-in one, call `startQRScan(...)` instead of the intent loop; keep the existing intent/browser fallback for the other options.
- Persist selection under existing `"scanner"` preference key.

## Risks / edge cases
- **Frame rate / CPU**: drop-latest channel prevents backlog; ~8 fps grayscale is fine for static QR. If too heavy, throttle in Java (re-`addCallbackBuffer` every Nth frame).
- **Permission denied**: show a message; fall back to existing external scanners.
- **Camera busy / no back camera**: `Camera.open` throws — catch, surface error, keep other scan options working.
- **Lifecycle leaks**: guarantee `release()` on stop/pause; never keep Camera across `onPause`.
- **JNI byte[] cost**: copying NV21 (~600 KB) per frame is acceptable at this fps; if needed, pass only a downscaled buffer or the Y plane.
- **Decode reliability**: gozxing handles skew/rotation via perspective transform; very small / very dark codes may still fail — non-blocking, user can retry or use external scanner.

## Validation
- `go build -tags=android ./...` and `go vet -tags=android ./...` pass.
- Desktop still builds (`go build ./...`) — stubs compile.
- On device (or emulator with simulated camera):
  - Grant CAMERA; point at a QR generated by the app's own QR section → decode → `textFromIntent` path fills the code.
  - Point at an `IO`-link QR (`https://abakum.github.io/croc#…`) → `uriFromIntent` path (same as browser-scan).
  - Scan under an angle → still decodes (gozxing perspective).
  - Cancel button + back/pause → camera released (verify via logcat, no "Camera being released" / leak warnings, re-open works).
  - Deny permission → graceful message, other scanner options unaffected.
- `make arm64` produces a signed APK that installs and runs the new option; `aapt2 dump badging` shows the CAMERA permission/feature.

## Out of scope
- CameraX / AndroidX (impossible without forking the fyne CLI — see above).
- Front-camera / torch / zoom controls.
- iOS camera scanning.
- Replacing the existing external intent / html5-qrcode scanners (kept as fallback options).

---

## Follow-up fix: gray-gradient preview = no frames reaching the viewfinder

### Symptom (on-device test)
Camera permission granted, scan dialog opens, but the viewfinder shows a **gray gradient** instead of live camera. That gradient is the initial `qrPreview` placeholder (2×2 RGBA with 2 transparent pixels, scaled up with bilinear filtering) **never being replaced by a frame** → frames are not reaching `qrPreview.Image = g`. Decode is irrelevant until frames flow.

### Root-cause hypotheses (logcat discriminates them)
- (a) `startCamera` failed → `Java: startCamera failed…` / status `Camera unavailable`.
- (b) `startCamera` OK but `onPreviewFrame` never fires — **most likely**: Camera1 `startPreview()` requires a surface target (`setPreviewDisplay`/`setPreviewTexture`); without one the capture pipeline doesn't start on many devices, so no preview callbacks. We intentionally have no `SurfaceView`.
- (c) frames flow but `cameraFrame` throws / returns false every time → `Java: cameraFrame threw…`.

Emulator test is the key discriminator: emulator reliably delivers preview frames, so if the emulator also shows gray → it's a code bug (JNI/cgo/preview update), not device-specific no-surface.

### Current logcat insufficiency (audited)
- Java logs all reach logcat (tag `croc`).
- Go: only `LogD()` reaches logcat; `log.Debugf` (schollz) goes to the in-app LOG tab only (`main.go` setOut(GUI)).
- **Gaps**: `onPreviewFrame` has **no** logging; `cameraFrameReceived` has **no** logging; `qr: startCamera failed`/`qr: stopCamera` use `log.Debugf` (`qr_camera.go:213,252`) → invisible in logcat.
- So today's logcat shows `Java: startCamera WxH` then silence → cannot distinguish (b) from (c).

### Ordered fix tasks

#### 1. Java `GoNativeActivity.java` — feed preview frames without a view
- Before `c.startPreview()`, add a headless surface target so the capture pipeline starts and `onPreviewCallbackWithBuffer` fires without a `SurfaceView`:
  ```java
  try { c.setPreviewTexture(new android.graphics.SurfaceTexture(0)); }
  catch (Throwable t) { Log.e(TAG, "Java: startCamera setPreviewTexture failed: " + t.getMessage()); }
  ```
  (Standard headless-capture trick; works on most devices. If a device still sends no frames, escalate to Camera2 + `ImageReader` which delivers YUV frames with no display surface.)
- Add a frame counter + periodic log inside `onPreviewFrame` (tag `croc`), e.g. log every 30th frame and the first one: `if (++qrFrameCount == 1 || qrFrameCount % 30 == 0) Log.d(TAG, "Java: onPreviewFrame #" + qrFrameCount + " " + w + "x" + h);` — this is the primary signal that the camera delivers frames.

#### 2. Go `qr_camera.go` — diagnostics that reach logcat
- In `cameraFrameReceived`, add periodic `LogD` (first frame + every ~1 s) so we confirm frames cross into Go: e.g. `LogD(fmt.Sprintf("qr: frame %dx%d #%d", w, h, n))` on first and every Nth call.
- Convert the two diagnostics to `LogD` so they show in logcat:
  - `qr_camera.go:213` `log.Debugf("qr: startCamera failed: %v", err)` → `LogD("qr: startCamera failed: " + fmt.Sprint(err))`
  - `qr_camera.go:252` `log.Debugf("qr: stopCamera: %v", err)` → `LogD("qr: stopCamera: " + fmt.Sprint(err))`
  (Use `LogD`, not `log.*`, for anything that must appear under `make logcat`.)

#### 3. Solid placeholder (cosmetic + avoids misleading gradient)
- Replace the initial 2×2-with-transparent-pixels image with a fully-opaque solid black image (e.g. fill all pixels black, or a small `image.NewUniform(color.Black)`-equivalent opaque raster) so the pre-camera state is a clean black square, not a gradient.

### Reading the logcat after the fix
`make logcat` (filters tag `croc`):
- `Java: startCamera 640x480` + `Java: onPreviewFrame #1 #30 #60…` + `qr: frame 640x480 #1…` → frames flow end-to-end; then either `qr: decoded …` (works) or no decode (gozxing/format issue → next-level).
- `Java: startCamera WxH` but **no** `onPreviewFrame` → confirms (b) no-surface; `setPreviewTexture` should fix — if still absent, escalate to Camera2+ImageReader.
- `onPreviewFrame` present but **no** `qr: frame` → broken JNI/cgo frame path (`Java_…_cameraFrame` / `cameraFrameNotify` / `cameraFrameReceived`) — inspect `cameraFrame threw` and the C bridge.

### Validation
- Rebuild (`make wsl arm64`), install on device + run on emulator.
- `make logcat`: confirm frame-counter lines appear; confirm `qr: decoded` on a real QR.
- Viewfinder shows live grayscale camera, not the gradient.
- Cancel/back/pause still release the camera (no leak).

---

## Follow-up fix #2: dialog hangs after the activity is paused/stopped

### Symptom (on-device, confirmed via logcat)
Preview rotation is now correct (viewfinder upright). New problem: when the activity is paused/stopped (e.g. user backgrounds the app for ~22 s), the camera is released (`Java: onPause` → `Java: stopCamera`) but the **Fyne scan dialog stays open with a frozen preview** (`qrPreview` never updated). Relevant logcat:
```
Java: onPause → lifecycle: pause → Java: stopCamera → Java: onStop → lifecycle: stop
... (~22 s) ...
Java: onRestart → lifecycle: restart/start → Java: onResume → lifecycle: resume   ← dialog still hung
```

### Root cause
- `GoNativeActivity.onPause()` (line 1209-1213) **always** calls `stopCamera()` for *every* pause, but `qr_camera.go` **never subscribes to lifecycle events**, so the Fyne dialog stays open with a dead camera.
- Constraint: `lifecycleFromJava` (`for_android.go:1066`, buffered cap 10) has a **single consumer** — the `select` loop in `send.go:1617`. QR lifecycle handling must be added **inside that send.go switch**.

### Design decision: close the dialog on pause (no resume logic)
Per the user's direction: on activity change, **close the scan dialog**, which in turn stops the camera (`stopQRScan` already calls `callVoid("stopCamera")` + `qrHideDialog()`). **No resume/restart on foreground** — the scan is abandoned; the user re-opens it to scan again.

### The permission-dialog trap (must guard)
`requestCameraPermission()` (`GoNativeActivity.java:300-308`) calls `requestPermissions(...)`, which shows the **system permission dialog → triggers `onPause`**. So closing on *every* `pause` would tear the scan down during the first-run permission request (before permission is granted). 

Guard: only close when the **camera is actually running** (`qrCameraStarted`). The camera can only start *after* permission is granted (`qrBegin` is reached only after the `hasCameraPermission()` check in `startQRScan`). Therefore `qrCameraStarted == true` ⟹ permission already granted ⟹ no permission dialog ⟹ closing on that pause is safe. The first-run permission-request pause is naturally skipped because `qrCameraStarted` is still `false`.

### Ordered task list

#### 1. Go `qr_camera.go` — running flag + pause handler
- Add `qrCameraStarted bool` to the `var (...)` block (`qr_camera.go:33`). Access it under `qrMu`.
- In `qrBegin` (`qr_camera.go:246`): on `startCamera` success (`ok && err == nil`), set `qrCameraStarted = true` (under `qrMu`).
- In `stopQRScan` (`qr_camera.go:278`): set `qrCameraStarted = false` inside the `active` block (after the early `if !active` return), alongside the existing teardown.
- Add helper `qrLifecyclePause()` (called from the lifecycle goroutine, **off the Fyne thread**):
  ```go
  func qrLifecyclePause() {
      qrMu.Lock()
      close := qrActive && qrCameraStarted
      qrMu.Unlock()
      if close {
          // stopQRScan touches UI (qrHideDialog) → run on the Fyne thread
          fyne.Do(func() { stopQRScan() })
      }
  }
  ```
- **No resume handler.** Do not add `qrLifecycleResume`, `qrResumeNeeded`, or `qrWindow` — explicitly out of scope per the user.

#### 2. Go `send.go:1617` — dispatch `pause` to QR
- In the `switch event` inside the `if isAndroid { go func() { for { select { case event := <-lifecycleFromJava:` block, add a case:
  ```go
  case "pause":
      qrLifecyclePause()
  ```
- Leave the existing `case "resume":` (line 1619) untouched (no QR resume). Do not add `qr*` calls to `stop`/`destroy` (the camera is released by Java's `onPause` already; `stopQRScan` runs on `pause`).

### Why this is correct
- **Has-permission path (the reported bug):** scan active, camera running (`qrCameraStarted == true`) → user backgrounds → `pause` → `qrLifecyclePause` closes the dialog + stops the camera. No hang. ✓
- **First-run permission request:** `requestPermissions` → `pause`, but `qrCameraStarted == false` → `qrLifecyclePause` does nothing; dialog stays so the poller can start the camera after grant. ✓
- **Permission denied (first run):** dialog stays showing "Camera permission denied"; the user cancels manually (or it stays — acceptable, no camera resource held since one never started). ✓
- **Thread safety:** `qrLifecyclePause` runs on the lifecycle goroutine; it only reads flags (under `qrMu`) and queues `stopQRScan` via `fyne.Do`, so `stopQRScan`'s UI access (`qrHideDialog` → `Dialog.Hide`) happens on the Fyne thread. `callVoid("stopCamera")` uses `driver.RunNative` and is thread-safe.
- **Idempotency:** `stopQRScan` is already idempotent (`active` flag + early return); `Java.startCamera`/`stopCamera` are idempotent (`qrCamera != null` checks). `Dialog.Hide()` firing `SetOnClosed → stopQRScan` again is a no-op.

### Validation
- `make amd64 adb` (or `make wsl arm64`) — build green (Go android + Java compile).
- `make logcat` with the dialog open: background the app → expect `Java: onPause` / `lifecycle: pause` / `Java: stopCamera`, and the dialog is gone on return (no frozen preview). Returning to the app does **not** auto-reopen the scan (user re-taps the option).
- First-run permission flow: grant → camera starts and scans; deny → "Camera permission denied" shown, dialog not torn down by the permission-request pause.
- Decoded QR still routed (`textFromIntent` / `uriFromIntent`); Cancel still releases; no `Camera is being released` warnings on re-open.

---

## Follow-up fix #3: built-in scan from recv jumps to settings

### Symptom (confirmed by user)
Scanning from the **recv** tab with the built-in camera selected jumps the active tab to **settings** (QR section) after decode — even though the recv "secret" entry is filled correctly. With a non-built-in (intent) scanner there is **no** jump.

### Root cause
`qr.Show()` (`applinks.go:277`) = `at.SelectIndex(3)` (settings) + open the QR accordion. It is invoked from the built-in scan `onResult` (`applinks.go:446`), which `qr.scanner()` (`applinks.go:434`) passes to `startQRScan`. The recv scan button (`recv.go:116` `qrButton`, ViewFullScreenIcon → `qr.scanner()`) and the settings QR section's own "Scan QRs" button (`applinks.go:426`) **share** this `onResult`. For the settings QR section the jump is intended (display the scanned code as QR); for recv it is not.

The decoded text reaches the recv flow **without** the callback: `qrRoute` (`qr_camera.go:133`) always feeds `uriFromIntent`/`textFromIntent`, consumed by the send-tab handler (`send.go:1672`) which does `entry.SetText(st)` + relay prefs — this is what fills the recv secret. So `qr.Show()` in `onResult` is pure (unwanted) navigation, not part of the secret-fill.

### Out of scope (do NOT change)
- **Send copy button (`send.go:299` `cbButton`):** its `qr.Show()` is intended — the button copies the code **and** displays its QR for the peer to scan. Leave as-is.
- `qr.scanner()` (`applinks.go:434`): unchanged — still used by the settings QR section (intended jump + `qr.currentText` display).
- `qrRoute` routing (`qr_camera.go:133`): unchanged; still fills the recv secret via `uriFromIntent`/`textFromIntent`.
- Send link tap on mobile (`send.go:779` `qr.Show()`) and `applinks.go:515` (intent fallback `qr.Show()`): intended, leave as-is.

### Fix

#### `recv.go:116` — built-in path scans without jumping to settings
Route the built-in case straight to `startQRScan` with a **no-op callback** (the secret is filled by `qrRoute`, which runs regardless of the callback; `qrRoute` already guards `cb != nil`):
```go
qrButton := widget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), func() {
	if (isAndroid || asMobile) && a.Preferences().String("scanner") == builtinScanner {
		startQRScan(a, w, nil) // no qr.Show(): stay on recv; secret filled via qrRoute
		return
	}
	if qr != nil {
		qr.scanner() // desktop (OpenURL) or intent fallback — no jump (unchanged)
	}
})
```
- `w` is recv's window (= the single main window), so the camera dialog still overlays recv. `startQRScan(a, w, nil)`: `qrOnResult = nil`; `qrRoute` skips `cb`; dialog still closes on decode.
- `qr.scanner()` (with its `qr.Show()` `onResult`) is left untouched — the settings QR section's "Scan QRs" button still jumps (intended).

### Validation
- `make amd64` green.
- recv, built-in selected: scan → secret entry filled, **stay on recv** (no settings jump); camera dialog closes on decode.
- recv, non-built-in scanner: unchanged (no jump, as before).
- Settings QR section "Scan QRs": still jumps to settings and shows the scanned QR (unchanged).
- Send copy button (`send.go:299`): unchanged — still copies the code and shows its QR (`qr.Show`).
