# Native Android camera surface for the built-in QR scanner

## Status: PLANNED — ready for implementation

## Problem
On Android 9/10 the built-in QR scanner **freezes** and runs at a strange FPS
(~15 fps despite a `30000-30000` request). Two root causes, both in the current
Camera1 + headless-capture design:

1. **Dummy surface stall.** `startCamera` feeds the camera a `new
   SurfaceTexture(0)` that is **never consumed** (`updateTexImage` is never
   called) — `GoNativeActivity.java:468`. On old Camera1→Camera2 HALs (Android
   9/10) an unconsumed surface eventually blocks the capture pipeline → preview
   freezes / frames stop. The only thing keeping it alive at all is that the
   preview *callback* path is partly independent of the surface path.
2. **Fyne-rendered preview overload.** Every frame is copied NV21 → JNI →
   `C.GoBytes` → Go `make+copy` (redundant 2nd copy) → rotated 0–3× in Go (each
   rotation allocates ~230 KB) → `fyne.Do` + `canvas.Image` refresh on the GL
   thread. At ~15 fps that is ~10–20 MB/s of garbage + GL-thread churn → GC
   stalls block the synchronous cgo `cameraFrame` call on the camera thread →
   jitter/freeze. The visible preview is also a choppy grayscale canvas, not a
   smooth color picture.

The prior Stage-1 fix (`20260709-qr-frame-rotation.md` "Android-9 low fps")
set `setPreviewFpsRange` (highest max, tie → larger min) and primed 3 buffers.
It raised delivery 3→~15 fps but the fixed `30000-30000` range + dummy surface
are still the bottleneck. The deferred Stage 2 was a full **Camera2 +
ImageReader** rewrite.

## Insight (user direction)
Render the preview in a **native Android window**, not Fyne. Giving the camera
a **real, consumed** surface makes the pipeline deliver full-rate color preview
reliably and removes the dummy-surface stall — so the **Camera2+ImageReader
rewrite is no longer needed**; we stay on Camera1. The Y plane still flows to
gozxing for **decode only**, which lets us delete the entire Fyne preview path.

## Decision: host = full-screen framework `Dialog` + `TextureView`
- **`TextureView` (not `SurfaceView`):** it lives in the normal view hierarchy,
  so there is no separate-window z-order/punch-through problem when hosted in a
  `Dialog` (a known SurfaceView-in-Dialog gotcha). It reuses the existing
  `Camera.setPreviewTexture(SurfaceTexture)` API — but with a surface the
  `TextureView` actually consumes (the view composites its own updates), so the
  pipeline never stalls. Falls back to `SurfaceView` only if a device shows a
  black TextureView (unlikely; note in plan).
- **`Dialog` over the `NativeActivity`** (not a separate Activity): no manifest
  change, no extra lifecycle; built **programmatically** (fyne's android build
  ships only `GoNativeActivity.java` + `android.jar` — no layout XML, no
  AndroidX, exactly the constraint in `20260708-builtin-qr-camera-scan.md`).
- **Color preview** for display; **grayscale Y → gozxing** for decode.

## Architecture / data flow (new)
```
startQRScan (Go)
  └─ permission OK? ── no ──▶ request; on grant (poll) show camera dialog
        │ yes
        ▼
  callVoid("showCameraDialog")  (Go→Java, RunNative)
        │
  Java (UI thread): build full-screen Dialog: TextureView + "Cancel" + hint text; show()
        │ TextureView.SurfaceTextureListener.onSurfaceTextureAvailable(st,…)
        ▼
  Java camera thread (HandlerThread "qrCamera"): Camera.open(back); configure
  (NV21, 640×480, focus, setDisplayOrientation); setPreviewTexture(st);
  setPreviewCallbackWithBuffer; prime 3 buffers; startPreview
        │ per frame (throttled ~10 fps): feedSquareFrame(squareY) → native cameraFrame
        ▼
  Go cameraFrameReceived (decode only): *image.Gray from Y (no 2nd copy) → qrDecodeCh (drop-latest)
        ├─ qrDecodeWorker: throttled qrRot → rotateGray90cw → gozxing.Decode
        │     └─ hit: qrStop; qrFinish(); callVoid("dismissCameraDialog"); qrRoute(text)
        └─ (no Fyne preview loop, no canvas.Image)
```
Stop/dismiss paths (all idempotent):
- **Decode success (Go):** `dismissCameraDialog()` (stops+releases on camera
  thread, dismisses on UI thread) + `qrFinish()` + route text.
- **Cancel button / hardware Back (Java):** `dismissCameraDialog()` then
  `lifecycleEvent("qrCancel")` → Go `qrLifecycleCancel()` → `qrFinish()` (no
  Java call; Java already dismissed).
- **Activity `pause` (Go lifecycle):** `qrLifecyclePause()` → if dialog active,
  `dismissCameraDialog()` + `qrFinish()`. Guarded so the first-run
  *permission-request* pause (dialog not yet shown) is a no-op.

## Contract: no new C/JNI
- `showCameraDialog` / `dismissCameraDialog` → static **void** methods on
  `GoNativeActivity` → Go `callVoid` (generic bridge, `for_android.go:39`).
- Cancel signal: reuse `lifecycleEvent("qrCancel")` (`GoNativeActivity.java:59`
  native decl → `for_android.c:454` → `lifecycleEventNotify` →
  `lifecycleFromJava`, consumed at `send.go:1617`). Already the pattern for
  `cameraPermissionGranted` etc.
- Decode frames: unchanged `cameraFrame(byte[],w,h)` → `for_android.c:480` →
  `cameraFrameNotify` → `cameraFrameReceived`.

## Ordered task list

### 1. Java — `GoNativeActivity.java`
- Add a dedicated camera thread so `Camera.open()`/config/`startPreview`/release
  never run on the UI or GL thread (current `startCamera` runs wherever
  `callBoolean` lands = the fyne native thread → latent ANR/visual freeze):
  ```java
  private static HandlerThread qrCameraThread;   // start once, looper
  private static Handler qrCameraHandler;
  private static Dialog qrDialog = null;
  private static TextureView qrTexture = null;
  private static volatile boolean qrDialogShown = false; // guards lifecycle dismiss
  ```
- `static void showCameraDialog()` (run body on UI thread via
  `runOnUiThread`): build a full-screen `Dialog` (programmatic): `FrameLayout`
  with a `TextureView` (match-parent) + a bottom `Button` "Cancel" + a `TextView`
  hint (`lp("Point at a QR code")` / fallback literal). `setCancelable(true)`;
  `setOnCancelListener` and the Cancel button → `dismissCameraDialog()` +
  `lifecycleEvent("qrCancel")`. Attach a `TextureView.SurfaceTextureListener`:
  on `onSurfaceTextureAvailable(st, w, h)` post `startCameraOnThread(st)` to
  `qrCameraHandler`. `onSurfaceTextureDestroyed` → return true (let dismiss
  handle teardown). Show the dialog, set `qrDialogShown=true`.
- `private static void startCameraOnThread(SurfaceTexture st)` (runs on
  `qrCameraHandler`): the existing open+configure logic (NV21, pick ≤640×480,
  continuous focus, **`setPreviewFpsRange`** — see task 2) + **`setPreviewTexture(st)`**
  (real consumed surface, replaces the dummy `SurfaceTexture(0)`) +
  `setDisplayOrientation(portrait)` + `setPreviewCallbackWithBuffer` + prime 3
  buffers + `startPreview`. Wrap in try/catch; on failure `dismissCameraDialog()`
  + `lifecycleEvent("cameraOpenFailed")` (Go shows "Camera unavailable"). Keep
  `qrPreviewWidth/Height`, `qrSquareBuf`, `feedSquareFrame` as-is.
- `static void dismissCameraDialog()` (idempotent): set `qrCameraRunning=false`,
  `qrDialogShown=false`; on `qrCameraHandler` post `stopCamera()` (existing:
  callback null, stopPreview, release, null `qrCamera`); on UI thread dismiss +
  null `qrDialog`. Safe to call when nothing is open.
- `onPreviewFrame` (`GoNativeActivity.java:260`): keep, but **throttle decode
  feed to ~10 fps** (time-based gate before `feedSquareFrame`) — preview is now
  native, decode needs only a few fps; this cuts JNI traffic + GC ~3×. Keep the
  `keep=false` → stop branch.
- `onPause` (`GoNativeActivity.java:~1280`): replace the bare `stopCamera()`
  safety call with `dismissCameraDialog()` (covers a frozen-dialog case
  directly). Keep best-effort try/catch.

### 2. Java — FPS range (fix the "strange FPS")
- In the params block: instead of "highest max, tie → larger min" (which forces
  the fragile fixed `30000-30000`), pick the range with the **highest max** and,
  among those, the **smallest min** (most flexible — lets AE adapt; e.g.
  `15000-30000`). try/catch as today. Log `Java: previewFpsRange <min>-<max>`.
  With a real surface the device will deliver up to the max reliably.

### 3. Go — `qr_camera.go` (android): decode-only pipeline, drop Fyne preview
- Remove: `qrPreview *canvas.Image`, `qrFrameCh`/`cameraFrame` struct,
  `qrPreviewLoop`, `qrShowDialog`, `qrHideDialog`, the black placeholder,
  `qrBegin` (replaced). If `qrSize` becomes unused, remove it (check).
- `cameraFrameReceived`: drop the redundant `make+copy` — `data` is already a
  fresh owned `C.GoBytes` slice, so `g := &image.Gray{Pix: data[:w*h], Stride: w,
  Rect: image.Rect(0,0,w,h)}`; send to `qrDecodeCh` (cap 1, drop-latest). Keep
  the periodic frame log (use `LogD` so it reaches logcat).
- `qrDecodeWorker`: per frame, throttled `qrRot` via `getDeviceRotation`
  (every ~250 ms, sticky on error, default from `qrSensorOrient`) →
  `rotateGray90cw` 0–3× → `gozxing.Decode`. On hit: `qrStop.Store(true)`;
  `qrFinish()`; `callVoid("dismissCameraDialog")`; `qrRoute(text)`.
- `startQRScan(a, w, onResult)`: same signature. Set up reader, `qrDecodeCh`,
  `qrSensorOrient`, launch `qrDecodeWorker`. If `hasCameraPermission()` →
  `callVoid("showCameraDialog")`; else `requestCameraPermission()` + existing
  poll; on grant → `callVoid("showCameraDialog")`; on deny/timeout → Fyne
  message `lp("Camera permission denied")`.
- `qrFinish()` (new, idempotent): under `qrMu` set `qrActive=false`,
  `qrCameraStarted=false`; `qrStop.Store(true)`; cancel ctx; `qrOnResult=nil`.
  Does **not** call Java (caller decides dismiss).
- `stopQRScan()` (public, unchanged signature): `qrFinish()` +
  `callVoid("dismissCameraDialog")`. (Keep for any external/edge caller.)
- `qrLifecyclePause()`: if `qrActive && qrCameraStarted` →
  `callVoid("dismissCameraDialog")` + `qrFinish()` (via `fyne.Do` not needed —
  no Fyne UI touched now; callVoid is `driver.RunNative`-safe).
- `qrLifecycleCancel()` (new): `qrFinish()` only (Java already dismissed +
  stopped). Set `qrCameraStarted=false` at `showCameraDialog` time instead of
  `qrBegin`.

### 4. Go — `send.go:1617` (lifecycle switch)
- Add `case "qrCancel": qrLifecycleCancel()`.
- Add `case "cameraOpenFailed":` → Fyne message `lp("Camera unavailable")` +
  `qrFinish()` (camera dialog never came up; no dismiss needed).
- Leave `case "pause": qrLifecyclePause()` (`send.go:1635`) as-is (its body
  changed in task 3).

### 5. Go — `for_android0.go` (non-android stubs)
- Keep `startQRScan` / `stopQRScan` / `qrLifecyclePause` no-op signatures; add
  `qrLifecycleCancel` and `qrFinish` no-ops (or guard the new send.go cases
  behind `isAndroid` — the switch is already inside `if isAndroid`).

## Failure modes / risks
- **Camera open on wrong thread** → ANR/visual freeze. Mitigation: all
  open/config/start/release on `qrCameraHandler`; only Dialog show/dismiss on UI
  thread. (This also removes the current latent on-native-thread open.)
- **TextureView black on an odd device** → documented fallback to `SurfaceView`
  + `SurfaceHolder` (`setPreviewDisplay`); revisit only if reported.
- **Permission-request pause** triggers `onPause`: guarded by `qrCameraStarted`
  (still false until `showCameraDialog`) → no-op, dialog not yet shown.
- **Double stop** (decode success + `onPreviewFrame` keep=false + Java cancel):
  all teardown paths are idempotent (`qrCamera!=null` checks, `qrActive` flag,
  `qrFinish` early-out).
- **`stopCamera` from `onPreviewFrame`'s keep=false** spawns a thread today
  (`GoNativeActivity.java:285`); harmless with idempotent `dismissCameraDialog`.
- **Decode reliability**: unchanged (gozxing + perspective). Throttling to
  ~10 fps does not hurt QR detection (static target).

## Validation
- `make wsl arm64` (or `make amd64 adb`) — Go android build green; Java compiles
  against `android.jar` only (no AndroidX refs).
- Desktop still builds (`go build ./...`) via stubs.
- `make logcat` on Android 9 and 10:
  - `Java: previewFpsRange …` shows a flexible range (e.g. `15000-30000`); the
    visible preview is **smooth color** at near the set max — no freeze.
  - `Java: onPreviewFrame #N` rises steadily; `qr: frame … #N` (decode feed)
    rises at ~10 fps; no gaps/stalls over 30+ s.
  - Point at a QR → `qr: decoded …` → `dismissCameraDialog` → recv secret /
    IO-link filled (`uriFromIntent`/`textFromIntent`); camera released (no
    "Camera is being released" warnings; re-open works).
  - Cancel button + hardware Back → `lifecycle: qrCancel` → dialog gone, camera
    released.
  - Background the app (pause ~20 s) → `lifecycle: pause` → dialog dismissed,
    no frozen overlay on return; scan not auto-reopened.
  - First-run permission: grant → camera starts; deny → "Camera permission
    denied", other scanner options unaffected.
- recv built-in scan (`recv.go:118`) stays on recv tab (no settings jump) —
  unchanged behavior, just native preview now.

## Out of scope
- Camera2 / ImageReader / CameraX (no longer needed — real surface removes the
  stall; CameraX impossible per `20260708` anyway).
- Front camera / torch / zoom.
- iOS.
- Existing external intent / html5-qrcode scanner options (kept as fallback).
- A graphical center reticle/scanning frame (optional polish; only a hint label
  + Cancel button for now).

---

# Follow-up #1: black preview (emulator AND real device) — TextureView → SurfaceView

## Symptom (confirmed via logcat + visual, on emulator AND a physical phone)
After `make 386 adb`, opening the built-in scanner shows a black Dialog and no
frames. Logcat:
```
Java: showCameraDialog shown
… ~16 s of silence …
Java: dismissCameraDialog done
lifecycle: qrCancel          (user gave up and cancelled)
```
Notably ABSENT: `Java: startCamera setPreviewTexture(texture) ok`,
`Java: previewFpsRange …`, `Java: startCamera WxH`, `Java: onPreviewFrame #1`,
`Java: startCamera failed`, `cameraOpenFailed`.
Visual: **fullscreen black, with the Cancel button + "Point at a QR code" text
visible on top** (so the Dialog is full-screen and the preview view has a real
size — sizing is NOT the problem). Identical on a real phone, so it is NOT an
emulator-only / software-GPU issue. Manifest has HW-accel on (default, no
`hardwareAccelerated="false"`).

## Root cause (evidence-based; rules out the competing hypotheses)
`startCameraWithSurface` **never ran** — proven by the total absence of its logs
(including its *failure* logs). It is only callable from `onSurfaceTextureAvailable`,
so **`onSurfaceTextureAvailable` never fired**. Therefore:
- NOT "setPreviewTexture failing silently" — it is never *called*.
- NOT a "camera started before surface ready" race — the start is *gated* on the
  callback, which never fires.
- NOT sizing/HW-accel — the Dialog is fullscreen with controls visible on both
  emulator and phone.

A fullscreen `TextureView` only creates its `SurfaceTexture` via the window's
**hardware-accelerated view-rendering pipeline (RenderThread)**. This app is a
Fyne/NativeActivity that renders the whole UI through its **own OpenGL ES context
(Fyne's renderer is a `GLSurfaceView`)**; in that setup a `TextureView` placed in
a child `Dialog` window never gets a produced surface → no callback → black, on
every device. (This is the risk pre-listed in this plan's "Failure modes".)

## Decision
Replace `TextureView` (+ `setPreviewTexture`) with **`SurfaceView` +
`SurfaceHolder.Callback`** (`setPreviewDisplay(holder)`).

Why `SurfaceView` is the correct (and high-confidence) fix here: a `SurfaceView`
creates its preview surface through a **dedicated native window managed by
WindowManager/SurfaceFlinger** — the *same mechanism Fyne itself uses*, because
Fyne's Android renderer **is a `GLSurfaceView` (which extends `SurfaceView`)**.
Since the app already renders successfully via that GLSurfaceView, SurfaceView
surfaces demonstrably work in this activity/Dialog context, whereas TextureView
surfaces do not. `SurfaceView` does not depend on the window's RenderThread
producing a SurfaceTexture layer, so it sidesteps the TextureView failure
entirely. Default z-order places the surface *behind* the host window, so the
hint `TextView` + Cancel `Button` (added after it in the `FrameLayout`) render on
top — matching the current "controls on top" layout. The rest of the design
(Dialog host, `HandlerThread`, FPS fix, ~10 fps decode throttle, lifecycle, Go
pipeline) is unchanged — **Go side untouched; Java-only edit.**

## Ordered tasks (Java — `GoNativeActivity.java`, all in `showCameraDialog` /
## `startCameraWithSurface`)

### 1. Imports
- Add `android.view.SurfaceHolder;` and `android.view.SurfaceView;`.
- Remove `android.graphics.SurfaceTexture;` and `android.view.TextureView;`
  (now unused) — verify no other references first (`grep`).

### 2. Field swap
- Replace `private static TextureView qrTexture = null;` with
  `private static SurfaceView qrSurface = null;` (or just drop it; it is only
  held to keep a strong ref — not strictly needed). Keep it minimal: drop the
  field, or store the `SurfaceView` for completeness.

### 3. `showCameraDialog()` — build a `SurfaceView` instead of `TextureView`
- Right after the window background block, add:
  ```java
  d.getWindow().setLayout(ViewGroup.LayoutParams.MATCH_PARENT,
                          ViewGroup.LayoutParams.MATCH_PARENT);
  ```
  (optional hardening — the Dialog is already fullscreen per the visual; this
  just guarantees it on every theme/device).
- Replace the `TextureView` + `SurfaceTextureListener` block with:
  ```java
  SurfaceView surface = new SurfaceView(act);
  surface.setLayoutParams(new ViewGroup.LayoutParams(
          ViewGroup.LayoutParams.MATCH_PARENT,
          ViewGroup.LayoutParams.MATCH_PARENT));
  SurfaceHolder holder = surface.getHolder();
  holder.addCallback(new SurfaceHolder.Callback() {
      public void surfaceCreated(SurfaceHolder h) {
          Log.d(TAG, "Java: surfaceCreated");
          startCameraOnThread(h);
      }
      public void surfaceChanged(SurfaceHolder h, int f, int w, int hh) {}
      public void surfaceDestroyed(SurfaceHolder h) {
          // dismiss is idempotent; covers surface loss without a Go teardown.
          dismissCameraDialog();
      }
  });
  ```
  Add `surface` to `root` FIRST (lowest z), then the hint, then Cancel (so they
  sit above the camera surface). Keep the existing hint/cancel/root layout.
- Update the assignment that stored `qrTexture = texture;` accordingly (or drop).

### 4. `startCameraOnThread` + `startCameraWithSurface` → holder-based
- Rename/repurpose to take a `SurfaceHolder`:
  `private static void startCameraOnThread(final SurfaceHolder h)` posting
  `startCameraWithHolder(h)`.
- In the open body replace:
  ```java
  c.setPreviewTexture(st);
  Log.d(TAG, "Java: startCamera setPreviewTexture(texture) ok");
  ```
  with:
  ```java
  try {
      c.setPreviewDisplay(h);
      Log.d(TAG, "Java: startCamera setPreviewDisplay ok");
  } catch (Throwable t) {
      Log.e(TAG, "Java: startCamera setPreviewDisplay failed: " + t.getMessage());
  }
  ```
  Everything else (open, NV21, ≤640×480, focus, FPS range, display orientation,
  square buffer, 3 callback buffers, `startPreview`, `failCameraOpen`) unchanged.

## Why this fixes it
- `SurfaceView` creates its surface through a dedicated native window
  (`TYPE_APPLICATION_MEDIA`) via WindowManager/SurfaceFlinger — independent of
  the window's RenderThread/SurfaceTexture pipeline that TextureView depends on.
  Fyne's own renderer is a `GLSurfaceView` (`SurfaceView` subclass) that renders
  fine in this app, proving this surface mechanism works here ⇒ `surfaceCreated`
  fires ⇒ camera opens ⇒ `setPreviewDisplay` + `startPreview` ⇒ full-rate color
  preview + `onPreviewFrame` for decode.
- The Go decode path is unchanged (still `feedSquareFrame` → `cameraFrame` →
  `cameraFrameReceived` → gozxing), so QR decoding, routing, cancel/pause all
  keep working identically.
- The `Java: surfaceCreated` log confirms the surface callback fires; if it ever
  does NOT, that single line tells us the problem is the surface/view (not the
  camera) — closing the inference gap from the black-preview logcat.

## Risks / edge cases
- **SurfaceView z-order in a Dialog:** default surface is *behind* the host
  window; the hint/cancel (normal views, added later) draw on top. If a device
  ever shows the button hidden behind the preview, call
  `surface.setZOrderMediaOverlay(true)` (keeps surface above the host's main
  layer but below any `setZOrderOnTop`). Do not add preemptively.
- **`surfaceDestroyed` → `dismissCameraDialog`:** idempotent; safe if it fires
  during normal dismiss (dismiss is already in flight).
- **Re-create on rotation:** not handled (the scan is abandoned on `pause`/config
  per the existing design; portrait-only is fine).
- **Diagnostic** `Java: surfaceCreated` stays so a future "black again" report is
  diagnosable in one logcat line (fires ⇒ surface OK, problem is camera; does not
  fire ⇒ surface/view issue).

## Validation
- `make 386 adb` (emulator) + `make logcat`:
  - `Java: showCameraDialog shown` → `Java: surfaceCreated` →
    `Java: startCamera setPreviewDisplay ok` → `Java: previewFpsRange …` →
    `Java: startCamera WxH` → `Java: onPreviewFrame #1 …` → live color preview.
  - Point the emulator camera at an on-screen QR → `qr: decoded …` →
    `dismissCameraDialog` → secret/IO-link filled.
  - Cancel / Back → `lifecycle: qrCancel`; pause → dialog dismissed.
- Confirm `grep` for `TextureView`/`SurfaceTexture` in `GoNativeActivity.java`
  returns nothing (imports cleanly removed).
- Real Android 9/10 device spot-check: same `surfaceCreated` → preview flow; no
  black, no freeze (the original target bug stays fixed).

---

# Follow-up #2: preview stretched + not rotated + rotation must track the phone

## Status: black-preview fixed (confirmed in logcat). Remaining display issues:
`surfaceCreated` → `setPreviewDisplay ok` → `previewFpsRange 2000-30000` →
`onPreviewFrame #1 … #300` now flow (SurfaceView worked). But the visible preview:
1. **Not rotated 90° CW** — shown sideways.
2. **Stretched** to fill the whole screen (640×480 / 4:3 stretched into the
   full-screen portrait SurfaceView).
3. **Must keep tracking device rotation** mid-scan (rotating the phone), like the
   pre-change code did.

## Pre-change rotation (git HEAD, commit `1c9dfda`) — what "worked with rotations"
The pre-change visible rotation was **Go-side, live**: `qrPreviewLoop` recomputed
`qrRot` from `getDeviceRotation()` every frame and rotated the Y plane before
drawing the Fyne canvas — so rotating the phone kept the preview upright. The
Java `setDisplayOrientation((info.orientation - 90 + 360) % 360)` (= **0°** for
orientation 90) was **visually inert**: it targeted the dummy `SurfaceTexture(0)`
(HEAD comment lines 328–331: "only affects SurfaceView/SurfaceTexture output").
Now that the preview is a real `SurfaceView`, rotation can ONLY come from
`setDisplayOrientation`, which must (a) use the correct value and (b) be re-applied
on rotation.

**Critical wrinkle:** the manifest declares
`configChanges="orientation|screenSize|…"`, so rotating the phone does **not**
recreate or pause the activity — it calls the existing `onConfigurationChanged`
(`GoNativeActivity.java:1425`). So "set once at camera open" would go stale on
rotation. The fix re-applies `setDisplayOrientation` (+ re-fits aspect) from
`onConfigurationChanged`, exactly mirroring how the Go code re-polled
`getDeviceRotation()` each frame.

## Root cause
- **Rotation value:** `(info.orientation - 90)` ⇒ 0° ⇒ sideways. Correct back-cam
  value is `(info.orientation - displayDegrees) % 360` where `displayDegrees` =
  `getDeviceRotation()` (0 portrait) ⇒ **90°** portrait. Same source the pre-change
  Go `qrRot` used (`qrRot = (sensorOrient - devRot)/90 = 1` portrait ⇒ 90° CW), so
  preview and decode agree (no Go change).
- **Stretch:** `SurfaceView` is `MATCH_PARENT` ⇒ camera scales 4:3 into the
  portrait screen ⇒ distorted. Size the SurfaceView to the preview's **displayed**
  aspect (3:4 portrait after the 90° rotation), centered, letterboxed.
- **Live rotation:** set once ⇒ stale after rotating; re-apply on
  `onConfigurationChanged`.

## Ordered tasks (Java-only — `GoNativeActivity.java`)

### 1. Cache sensor orientation (field, near the other `qr*` fields)
```java
// Back-camera sensor orientation (degrees). Cached at camera open so
// reapplyPreviewOrientation can recompute the display angle on rotation without
// re-querying CameraInfo.
private static int qrSensorOrientJava = 90;
```

### 2. Rotation at camera open (`startCameraWithHolder`, replace the
###   `(info.orientation - 90 …)` block, ~line 593)
First hoist a `rotate` local to method scope (so the `fitPreviewAspect` call near
the end of the method can see it): declare `int rotate = 90;` alongside
`int width = 0, height = 0;`. Then in the rotation `try/catch` block:
```java
qrSensorOrientJava = info.orientation;
int degrees = getDeviceRotation();                 // 0/90/180/270; -1 -> 0
if (degrees < 0) degrees = 0;
rotate = (info.orientation - degrees + 360) % 360;  // back-cam formula
c.setDisplayOrientation(rotate);
```
`getDeviceRotation()` is a plain `Display.getRotation()` read, safe on the camera
thread. `rotate` is now in scope for `fitPreviewAspect(width, height, rotate)`
called after `startPreview()` (task 3).

### 3. Aspect-ratio letterbox helper (new static method)
```java
// Letterbox the preview SurfaceView to the camera's displayed aspect ratio
// (setDisplayOrientation swaps w/h at 90/270), centered in its parent. Runs on
// the UI thread (after camera open and on each rotation).
private static void fitPreviewAspect(final int camW, final int camH, final int rotate) {
    final SurfaceView sv = qrSurface;
    if (sv == null) return;
    sv.post(new Runnable() {
        public void run() {
            android.view.ViewParent p = sv.getParent();
            if (!(p instanceof android.view.ViewGroup)) return;
            android.view.ViewGroup parent = (android.view.ViewGroup) p;
            int pw = parent.getWidth(), ph = parent.getHeight();
            if (pw <= 0 || ph <= 0) return;                 // not laid out yet
            boolean swap = (rotate == 90 || rotate == 270);
            float aspect = swap ? (float) camH / camW : (float) camW / camH; // displayed w/h
            int bw, bh;
            if ((float) pw / ph > aspect) { bh = ph; bw = (int) (ph * aspect); }
            else                          { bw = pw; bh = (int) (pw / aspect); }
            android.widget.FrameLayout.LayoutParams lp =
                    new android.widget.FrameLayout.LayoutParams(bw, bh);
            lp.gravity = Gravity.CENTER;
            sv.setLayoutParams(lp);
        }
    });
}
```
Call it at the end of `startCameraWithHolder` (after `startPreview()`):
`fitPreviewAspect(width, height, rotate);`

### 4. Re-apply on rotation (new static method + hook the existing
###   `onConfigurationChanged`)
```java
// Recompute + apply preview orientation and re-fit aspect for the current device
// rotation. Called at config changes (rotating the phone, which configChanges
// absorbs without recreating the activity) while the camera Dialog is up.
private static void reapplyPreviewOrientation() {
    final Camera c = qrCamera;
    if (c == null) return;
    int degrees = getDeviceRotation();
    if (degrees < 0) degrees = 0;
    final int rotate = (qrSensorOrientJava - degrees + 360) % 360;
    final int camW = qrPreviewWidth, camH = qrPreviewHeight;
    qrCameraHandler.post(new Runnable() {
        public void run() { try { c.setDisplayOrientation(rotate); } catch (Throwable ignored) {} }
    });
    fitPreviewAspect(camW, camH, rotate);
}
```
Extend the existing `onConfigurationChanged` (`:1425`):
```java
public void onConfigurationChanged(Configuration config) {
    super.onConfigurationChanged(config);
    updateTheme(config);
    if (qrDialogShown) reapplyPreviewOrientation();
}
```
(`onPreviewFrame` bytes are still sensor-native; `setDisplayOrientation` only
rotates the surface display, never the callback data — so Go's `qrRot` in
`qrDecodeWorker`, which re-polls `getDeviceRotation` per frame, continues to keep
decode upright independently. Both paths now track rotation like the pre-change
Go preview did.)

## Why this fixes all three
- Rotation value: `setDisplayOrientation(90)` in portrait ⇒ upright, matching the
  pre-change Go `qrRot=1` (90° CW).
- Stretch: centered 3:4 box, letterboxed by the black Dialog bg ⇒ undistorted.
- Live rotation: `onConfigurationChanged` re-applies orientation + re-fits aspect
  so rotating the phone mid-scan keeps the preview upright (parity with the
  pre-change per-frame Go rotation).

## Risks / edge cases
- **`setDisplayOrientation` thread:** posted to `qrCameraHandler` (the camera's
  owner thread) on rotation; safe. At open it's set inline on that same thread.
- **Resize fires `surfaceChanged`:** currently a no-op; camera keeps previewing
  and re-fits. No re-open (`startCameraWithHolder` guards `qrCamera != null`).
- **`parent` size 0:** guarded by `sv.post` (defers until laid out); if it stays 0
  the preview is just full-screen (cosmetic), non-fatal.
- **`getDeviceRotation` returns -1:** falls back to `degrees=0` (portrait).
- **Decode unaffected:** `onPreviewFrame` bytes + `qrRot` unchanged.

## Validation (the user is testing on an Android 9 emulator now)
- `make amd64 adb` (x86_64 emulator) + `make logcat`:
  - Preview **upright, undistorted** (3:4 portrait box, black bars top/bottom);
    Cancel + hint on top.
  - **Rotate the phone mid-scan** (emulator Ctrl+F11/F12) ⇒ preview stays upright
    and re-fits aspect (parity with pre-change behavior).
  - `Java: onPreviewFrame #N` rises steadily; point the emulated camera at a QR ⇒
    `qr: decoded …` ⇒ recv secret / IO-link filled.
  - Cancel / Back / pause ⇒ dismiss + release.
- **Android 9 performance** (the reason this whole change exists): with a real
  SurfaceView surface (no dummy-texture stall) + the ~10 fps decode throttle +
  preview off the Go/GL thread, expect smooth preview at the set rate with **no
  freeze**; confirm via `onPreviewFrame` cadence over 30+ s (no gaps/stall) and
  `previewFpsRange` honored.
- Cross-check on the physical arm64 phone (`make arm64 adb`) once available:
  upright, undistorted, live rotation, decodes, no freeze.

---

# Follow-up #3: dialog sized to the camera (no stretch, no fullscreen)

## Status: rotation confirmed working. Stretch remains (fullscreen `MATCH_PARENT`
## SurfaceView distorts the 4:3 camera into the portrait screen).
Goal (user): make the **Dialog window itself** the camera's displayed size/aspect,
centered and floating over the Fyne window (as already observed on API 31), so the
`SurfaceView` (still `MATCH_PARENT`) fills an aspect-correct window ⇒ no stretch,
and `fitPreviewAspect` is **not** needed (never added). The Android 9 emulator
crash is ignored (emulator-only).

## Approach (Java-only; explicit window sizing — Option B)
Replace the current fullscreen `getWindow().setLayout(MATCH_PARENT, MATCH_PARENT)`
(`GoNativeActivity.java:432`) with an explicit **camera-aspect** window size,
computed by contain-fitting the preview's displayed aspect (camW×camH after the
`setDisplayOrientation` rotation) within the screen, then centered. The
`SurfaceView` stays `MATCH_PARENT` and now fills an aspect-correct window ⇒ the
camera (4:3 → 3:4 portrait) is shown undistorted; the Fyne window is visible
around the centered box. Hint + Cancel stay overlaid (top/bottom), so the dialog
remains exactly the camera's size.

## Ordered tasks (Java — `GoNativeActivity.java`)

### 1. New helper `sizeDialogToCamera(camW, camH, rotate)` (static)
Contain-fits the camera's displayed aspect into the screen, then sizes + centers
the Dialog window. Must touch the window on the UI thread (callers ensure that).
```java
// Size + center the camera Dialog window to the preview's displayed aspect ratio
// (camW x camH after the setDisplayOrientation `rotate`), contain-fit in the
// screen. The MATCH_PARENT SurfaceView then fills an aspect-correct window => no
// stretch. UI-thread only.
private static void sizeDialogToCamera(int camW, int camH, int rotate) {
    final Dialog d = qrDialog;
    if (d == null || d.getWindow() == null || camW <= 0 || camH <= 0) return;
    boolean swap = (rotate == 90 || rotate == 270);
    float aspect = swap ? (float) camH / camW : (float) camW / camH; // displayed w/h
    android.util.DisplayMetrics dm = goNativeActivity.getResources().getDisplayMetrics();
    int sw = dm.widthPixels, sh = dm.heightPixels;
    int bw, bh;
    if ((float) sw / sh > aspect) { bh = sh; bw = (int) (sh * aspect); }
    else                          { bw = sw; bh = (int) (bw / aspect); }
    final Window w = d.getWindow();
    w.setGravity(Gravity.CENTER);
    w.setLayout(bw, bh);
}
```

### 2. `showCameraDialog()` — drop fullscreen, pre-size to the (assumed) camera
Replace the `setLayout(MATCH_PARENT, MATCH_PARENT)` block (`:432`) with:
```java
d.getWindow().setBackgroundDrawable(
        new android.graphics.drawable.ColorDrawable(Color.BLACK));
// Pre-size the window to the camera's displayed aspect so the SurfaceView is
// non-zero (needed for surfaceCreated) and undistorted from the first frame.
// Assume 4:3 (640x480 — the size our picker always selects); refined to the
// real size after open and on rotation.
int d0 = getDeviceRotation();
if (d0 < 0) d0 = 0;
sizeDialogToCamera(640, 480, (qrSensorOrientJava - d0 + 360) % 360);
```
(Keep `FEATURE_NO_TITLE` and the `SurfaceView`/hint/Cancel build unchanged.)

### 3. Refine to the real size at camera open (`startCameraWithHolder`)
**First hoist `rotate` to method scope** so it's visible here: change the rotation
block (implemented last turn) to declare `int rotate = 90;` alongside
`int width = 0, height = 0;` at the top of the method, and assign
`rotate = (info.orientation - degrees + 360) % 360;` inside the rotation `try`
(remove its `int` there). Then, after `startPreview()` succeeds, post to the UI
thread to size the dialog to the actual camera dimensions:
```java
final int rw = width, rh = height, rr = rotate;
goNativeActivity.runOnUiThread(new Runnable() {
    public void run() { sizeDialogToCamera(rw, rh, rr); }
});
```
(Usually 640×480 @ 90° → identical to the pre-size; no visible change.)

### 4. Re-size on rotation — extend `reapplyPreviewOrientation`
After recomputing `rotate` (it already re-applies `setDisplayOrientation`), also
resize the dialog so it tracks orientation (portrait 3:4 ↔ landscape 4:3):
```java
private static void reapplyPreviewOrientation() {
    final Camera c = qrCamera;
    if (c == null) return;
    int degrees = getDeviceRotation();
    if (degrees < 0) degrees = 0;
    final int rotate = (qrSensorOrientJava - degrees + 360) % 360;
    qrCameraHandler.post(new Runnable() {
        public void run() { try { c.setDisplayOrientation(rotate); } catch (Throwable ignored) {} }
    });
    sizeDialogToCamera(qrPreviewWidth, qrPreviewHeight, rotate); // UI thread (onConfigurationChanged)
}
```
(`onConfigurationChanged` runs on the UI thread, so the direct
`sizeDialogToCamera` call is safe there; `qrPreviewWidth/Height` are set at open.)

## Why this fixes the stretch
- The window becomes a centered, aspect-correct box (3:4 portrait). The
  `MATCH_PARENT` `SurfaceView` fills it, and the camera's 4:3 buffer — rotated to
  3:4 by `setDisplayOrientation(90)` — maps 1:1 onto that surface ⇒ undistorted.
- No `fitPreviewAspect`, no letterboxing inside a fullscreen dialog; the dialog IS
  the camera. The surrounding Fyne window stays visible (matches the observed
  API-31 floating behavior, now deliberate and correct on all APIs).
- Rotation: `setDisplayOrientation` (orientation) + `sizeDialogToCamera` (aspect)
  both update on `onConfigurationChanged`.

## Risks / edge cases
- **`setLayout` after `show()`:** WindowManager relayouts the window — fine; with
  the pre-size at show (task 2) the open-time refine (task 3) is usually a no-op,
  so flicker is minimal (only on actual rotation).
- **Dialog theme decoration/padding:** `FEATURE_NO_TITLE` + black bg minimize it;
  any residual inset just shrinks the SurfaceView slightly (still no stretch
  within it). If a device shows a visible border, switch the dialog to a
  no-frame theme or `setBackgroundDrawable(null)` (deferred).
- **`DisplayMetrics` vs real screen / insets:** `widthPixels`/`heightPixels` may
  exclude nav bar on some APIs; the contain-fit stays aspect-correct either way
  (just slightly smaller). Acceptable for a viewfinder.
- **Non-4:3 preview:** if a device lacks 640×480, the pre-size (4:3) differs from
  the real size for one frame, then task 3 corrects it. Rare.
- **Camera open on wrong thread / crash:** unchanged from Follow-up #1 (all
  camera ops on `qrCameraHandler`). Android 9 emulator crash stays out of scope.

## Validation
- `make amd64 adb` (x86_64 emulator) + `make logcat`:
  - Dialog is a **centered camera-aspect box** (3:4 portrait), Fyne visible around
    it, hint + Cancel overlaid; preview **undistorted**.
  - Rotate mid-scan ⇒ box re-sizes (3:4 ↔ 4:3) and preview stays upright.
  - `Java: onPreviewFrame #N` steady; QR decode + routing works; Cancel/Back/pause
    dismiss + release.
- Confirm on the API-31 device/emulator (where it was already floating): now
  deliberately camera-sized and undistorted, not wrapping the text.
- Physical arm64 phone spot-check: undistorted, live rotation, decodes, no freeze.
