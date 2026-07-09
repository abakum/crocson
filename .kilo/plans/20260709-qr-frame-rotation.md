# QR frame rotation per device orientation (+ Java-side Y crop)

## Context
Built-in QR scanner (Android, Camera1, NV21). Preview frames are rendered in
Fyne; the Y plane is also decoded by gozxing. `qr_camera.go:131-133` rotates each
cropped square frame `qrRot` times with `rotateGray90cw`. Currently `qrRot` is
computed **once** at scan start from the constant sensor orientation
(`qr_camera.go:218` → `qrRot = (o/90)%4`), so it is effectively a constant (≈1)
and is only correct for one holding orientation.

Data flow today: Java sends the **full NV21** buffer (`onPreviewFrame` → native
`cameraFrame(byte[], w, h)` → `for_android.c:480-489` → Go `C.GoBytes`) even
though only the Y plane is used; Go then copies Y and runs `cropCenterSquare`.
This ships ~460 KB/frame across JNI and triple-copies, for ~half of it discarded
(chroma).

Empirically the correct rotation depends on how the phone is held:
- camera on top (portrait) → 1
- camera on left (landscape CCW) → 0
- camera on right (landscape CW) → 2
- camera on bottom (reverse portrait) → not reached: the display/Fyne does not
  rotate there, so `Display.getRotation()` never yields 180.

## Root cause
The missing variable is the **live device rotation** (`Display.getRotation()`).
The sensor orientation is a hardware constant (~90° on virtually all back
cameras) and alone cannot track the user's grip. The correct, device-independent
count of 90° CW rotations to apply to the NV21 Y plane (the same clockwise angle
`setDisplayOrientation` would apply to a surface) is:

```
qrRot = ((sensorOrient - deviceRot + 360) % 360) / 90
```

Verified against all observed points (sensor = 90): top(0°)→1, left(90°)→0,
right(270°)→2.

## Decisions
- **Rotation stays in Go.** `rotateGray90cw` remains in `qr_camera.go`, so Go
  controls rotation and can add per-device overrides later without touching Java.
- **Crop + Y-extraction move to Java** (rotation-invariant: the centered square
  of the raw sensor-native frame is the same regardless of grip). Java extracts
  the Y plane and crops to the centered square, then sends only that. This halves
  JNI traffic (drops chroma) and removes redundant copies in Go.
- **No tablet/phone detection.** The rotation formula adapts via real sensor +
  real `Display.getRotation()` (relative to natural orientation). Per-device
  override deferred (out of scope).
- **No new C code.** The generic `callInt` bridge (`for_android.c:60`,
  `for_android.go:69`) already dispatches `()I` static methods, and the native
  `cameraFrame(byte[], w, h)` (`for_android.c:480-489`) forwards whatever
  `len`/`w`/`h` Java passes — so a cropped square Y array needs no C change.
- `getCameraSensorOrientation()` (`GoNativeActivity.java:327`) is kept and cached
  once at scan start (hardware constant, independent of grip).
- A new `getDeviceRotation()` is queried **per decoded frame** in `qrDecodeLoop`.

## Tasks
1. **`GoNativeActivity.java`** — add a thin static method near
   `getCameraSensorOrientation` (≈ line 338):
   ```java
   // Current display rotation in degrees (0/90/180/270), relative to the
   // device's natural orientation; -1 if unavailable. Used by Go to rotate the
   // NV21 Y plane so the QR preview/decode matches the screen orientation.
   static int getDeviceRotation() {
       try {
           if (goNativeActivity == null) return -1;
           int r = goNativeActivity.getWindowManager().getDefaultDisplay().getRotation();
           if (r == android.view.Surface.ROTATION_0)   return 0;
           if (r == android.view.Surface.ROTATION_90)  return 90;
           if (r == android.view.Surface.ROTATION_180) return 180;
           if (r == android.view.Surface.ROTATION_270) return 270;
           return -1;
       } catch (Throwable t) {
           Log.e(TAG, "Java: getDeviceRotation failed: " + t.getMessage());
           return -1;
       }
   }
   ```

2. **`GoNativeActivity.java`** — extract Y + crop to a centered square on the
   camera thread, reusing one buffer (no per-frame allocation):
   - In `startCamera`, after `qrPreviewWidth/Height` are set
     (`GoNativeActivity.java:405-406`), compute `side = min(w, h)` and allocate a
     reused `byte[side*side]` (store it in a static field, e.g. `qrSquareBuf`;
     `int qrSquareSide`). Reset it to null in `stopCamera`.
   - In `onPreviewFrame` (`GoNativeActivity.java:255-288`), before calling
     `cameraFrame(...)`: if `qrSquareBuf` fits, copy the centered square of the Y
     plane (`first w*h bytes`) into it and call `cameraFrame(qrSquareBuf, side,
     side)` instead of passing `data`. (The NV21 `data` is still recycled via
     `camera.addCallbackBuffer(data)` as today.) If the buffer is missing or the
     frame is smaller than expected, fall back to passing `data` unchanged (Go
     keeps `cropCenterSquare` as a safety net — see task 4).
   - Pseudocode for the copy (`data` = NV21, stride = `qrPreviewWidth`):
     ```java
     int side = qrSquareSide;
     int xoff = (qrPreviewWidth - side) / 2;
     int yoff = (qrPreviewHeight - side) / 2;
     for (int r = 0; r < side; r++) {
         int src = (yoff + r) * qrPreviewWidth + xoff;
         System.arraycopy(data, src, qrSquareBuf, r * side, side);
     }
     ```

3. **`qr_camera.go`** — add a package-level var `qrSensorOrient int` next to
   `qrRot`. Replace the `qrRot = (o/90)%4` block at `qr_camera.go:213-219`:
   ```go
   qrSensorOrient = 90 // safe default for back cameras
   if o, err := callInt("getCameraSensorOrientation"); err == nil && o >= 0 {
       qrSensorOrient = o
       logD(fmt.Sprintf("getCameraSensorOrientation %d", o))
   }
   qrRot = 0 // recomputed per frame
   ```

4. **`qr_camera.go` `qrDecodeLoop`** (`qr_camera.go:131-133`) — recompute `qrRot`
   each frame before rotating; keep previous value on error/-1:
   ```go
   if devRot, err := callInt("getDeviceRotation"); err == nil && devRot >= 0 {
       qrRot = (((qrSensorOrient - devRot) % 360) + 360) % 360 / 90
   } // else keep previous qrRot (sticky; default 0)
   ```
   The frame now arrives already cropped to `side×side` from Java, so **drop the
   `cropCenterSquare` call** from the normal path. Keep `cropCenterSquare` in the
   file as a fallback for the non-square fallback frame (task 2) — i.e. call it
   only when `w != h`. Keep the `rotateGray90cw` loop unchanged.

## Failure modes
- `getDeviceRotation` returns -1 / `callInt` errors → keep previous `qrRot`
  (sticky), default 0 at start. Worst case: one stale orientation until next good
  read; never a panic (`callInt` already clears JNI exceptions in
  `for_android.c:70-75`).
- `getCameraSensorOrientation` unavailable → default `qrSensorOrient = 90`
  (correct for ~all back cameras).
- Threading: `callInt` marshals via `driver.RunNative` (`for_android.go:71`), safe
  to call from the decode goroutine; `getRotation()` is a plain Display read.
- **Java crop buffer**: if `qrSquareBuf` is null or the frame is smaller than
  `side²`, fall back to passing `data` unchanged; Go keeps `cropCenterSquare`
  (guarded by `w != h`) as a safety net — no crash, just the old (less efficient)
  path. The reused buffer is allocated once in `startCamera` (preview size is
  fixed per scan) and nulled in `stopCamera`, so there is no per-frame allocation
  and no memory held between scans.

## Validation
- During testing, add a throttled (e.g. once per 30 frames) log inside the
  decode loop: `logD(fmt.Sprintf("qr: devRot=%d sensor=%d qrRot=%d", devRot,
  qrSensorOrient, qrRot))`; remove/tone down after.
- Build for android, open the scanner, and in each orientation confirm the
  **preview is upright and a QR decodes**:
  - camera on top → qrRot 1
  - camera on left → qrRot 0
  - camera on right → qrRot 2
- Rotate mid-scan (top→right→left): `qrRot` must update per frame; preview stays
  upright and decode works in each grip.
- Confirm the existing sensor log (`getCameraSensorOrientation %d`) still prints
  ~90 on the target phone.

## Out of scope
- Tablet/phone detection and per-class rotation branch (architecture supports a
  Go-side override later if a specific device misbehaves).
- The hardcoded `setDisplayOrientation` in `startCamera`
  (`GoNativeActivity.java:401-402`) — affects no visible surface (dummy texture
  only); leave as-is.
- Throttling `callInt("getDeviceRotation")`: per-decoded-frame is fine since
  gozxing decode dominates cost. Revisit only if profiling shows the native hop
  is a bottleneck.

---

# Follow-up: Android-9 low camera fps (~3 fps) — camera delivery rate

## Root cause (corrected)
The square delivered to Go is **480×480 on BOTH** Android 9 (~3 fps) and Android 14
(~23 fps) — i.e. the chosen preview is 640×480 on both; the `sizes.get(0)` fallback
(`GoNativeActivity.java:393`) never fired and there is **no 4K frame** (an earlier
4K hypothesis was disproven by the user reading the log). With an identical frame,
the 3-vs-23 gap is the **camera HAL delivering ~3 fps on Android 9** for the same
640×480. The Go pipeline (preview/decode decoupled in `qrPreviewLoop`/
`qrDecodeWorker`) is not the limiter. `setPreviewFpsRange` is **not set**
(`GoNativeActivity.java:380-398`) → the device uses its default range, which on old
HALs is low; only 1 callback buffer is primed.

## Stage 1 (primary fix — Java; isolate this variable)
Keep preview-size selection **as-is** (640×480 → 480 square) so the only change is
the camera rate. In `startCamera` params block (`GoNativeActivity.java:377-401`):

1. **FPS range** — query `getSupportedPreviewFpsRange()`, pick the range with the
   largest `max` (tie → larger `min`, preferring a fixed high rate), apply
   `setPreviewFpsRange`. try/catch (some devices reject ranges):
   ```java
   try {
       List<int[]> ranges = params.getSupportedPreviewFpsRange();
       int[] picked = null;
       if (ranges != null) for (int[] r : ranges) {
           if (r == null || r.length < 2) continue;
           if (picked == null || r[1] > picked[1] ||
               (r[1] == picked[1] && r[0] > picked[0])) picked = r;
       }
       if (picked != null) {
           params.setPreviewFpsRange(picked[0], picked[1]);
           Log.d(TAG, "Java: previewFpsRange " + picked[0] + "-" + picked[1]);
       }
   } catch (Throwable ignored) {}
   ```
2. **Buffers** — prime **3** callback buffers instead of 1; keep the per-frame
   re-add on `keep`:
   ```java
   int bufSize = Math.max(1, width) * Math.max(1, height)
       * ImageFormat.getBitsPerPixel(ImageFormat.NV21) / 8;
   for (int i = 0; i < 3; i++) c.addCallbackBuffer(new byte[bufSize]);
   ```
3. **Logging** — `Java: previewFpsRange <min>-<max>`; chosen size already logged by
   `Java: startCamera WxH` (`GoNativeActivity.java:426`).

No Go changes. The square size is already in the Go log (`qr: frame 480x480`,
every 30 frames).

## Validation
- `make wsl arm64`.
- On Android 9: `Java: previewFpsRange …` shows a high range; the frame counter
  (`Java: onPreviewFrame #N` / `qr: frame #N`) rises toward the set rate (~15–30
  fps); the square stays 480×480 (expected); a real QR decodes reliably.

## Deferred / escalation
- Size reduction (square ≤360) + safe `sizes.get(0)` fallback: deferred — the
  square is identical on both devices, so size is not the cause. Revisit (as a
  separate, isolated experiment) only if Stage 1 is insufficient and a smaller
  preview is suspected to ease the HAL.
- Go square-size logging: already present (`qr: frame WxH`); make it more
  prominent only if requested.
- Stage 2 (if Stage 1 insufficient): `camera2` + `ImageReader` (framework-only,
  API 21+), delivering frames at a set rate independent of any display surface.
