# Fix QR viewfinder darkening overlay on MIUI/HyperOS (Xiaomi, Android 14)

## Context / confirmed root cause
- **Symptom:** On a Xiaomi phone running Android 14 the QR camera `Dialog` shows only the
  camera preview edge-to-edge — the side/top-bottom shadow darkening and the white square
  border drawn by `QrOverlayView` are **completely invisible**. The camera preview itself is fine.
- **Scope:** Works on all emulators (including an API 34 emulator) and on Android < 14 phones.
  → Confirmed **MIUI/HyperOS-specific**, NOT an AOSP Android 14 change.
- **Mechanism:** `QrOverlayView` (GoNativeActivity.java:463–522) is a transparent `View`
  placed over the camera `SurfaceView` inside a sized `Dialog`. In its constructor it forces
  `setLayerType(View.LAYER_TYPE_HARDWARE, null)` on API ≥ Q (lines 472–474).
  `LAYER_TYPE_HARDWARE` promotes the view into a **separate compositor layer** (a TextureLayer
  / its own `SurfaceControl`). On MIUI/HyperOS that separate transparent layer is ordered
  **behind** the `SurfaceView`'s child surface, so the view's `onDraw` output (the shadow
  rects + border) is hidden by the camera preview. On AOSP the layer composites above the
  surface, which is why emulators / stock Android are unaffected.
- The reliable, canonical fix for "overlay View disappears above a SurfaceView" is to render
  the overlay **into the host window's own surface** (which the WindowManager always places
  above the `SurfaceView` surface) instead of a separate hardware layer. `LAYER_TYPE_SOFTWARE`
  does exactly that (draws into a Bitmap that HWUI blits into the window surface). Cost is
  negligible: a handful of static rects, redrawn only on size change.

## Files touched
- `GoNativeActivity.java` only.

## Fix steps
1. **Primary fix — overlay layer type.** In `QrOverlayView`'s constructor
   (GoNativeActivity.java:467–485), replace the API-≥-Q `LAYER_TYPE_HARDWARE` block
   (lines 472–474) with an unconditional software layer:
   ```java
   setLayerType(View.LAYER_TYPE_SOFTWARE, null);
   ```
   This removes the separate compositor layer that MIUI orders behind the camera surface, so
   the shadow/border draw into the Dialog window surface (always above the `SurfaceView`).
   - Do NOT use `LAYER_TYPE_HARDWARE` and do NOT gate on `Build.VERSION` — software is
     universally correct over a SurfaceView and the view is trivial to render.

2. **Defensive redraw (guard against a stale first draw).** The overlay is laid out when the
   Dialog window may not yet have its final size; `onSizeChanged` → `invalidate` may fire once
   with stale geometry. Store the overlay in a field (e.g. `private static QrOverlayView qrOverlay`)
   in `showCameraDialog` (GoNativeActivity.java:606) and, after `d.show()` (line 625), post:
   ```java
   qrOverlay.post(() -> { qrOverlay.requestLayout(); qrOverlay.invalidate(); });
   ```
   Also null the field in `dismissCameraDialog` where `qrSurface = null` is set
   (GoNativeActivity.java:893 and :902).
   - Optional: call `overlayView.bringToFront();` right after `root.addView(overlayView);`
     (line 612) so the overlay is guaranteed the top child of the `FrameLayout` (it already is,
     as the last added child, but this is explicit/harmless).

3. **Leave everything else unchanged.** No changes to camera open/config, preview sizing,
   `setDisplayOrientation`, decode path, dialog sizing math, or lifecycle. The `setZOrderOnTop`
   / `setZOrderMediaOverlay` lines stay commented (default SurfaceView z-order — surface below
   the host window — is what we want).

## Fallback (only if step 1 still doesn't show on Xiaomi)
- Replace `LAYER_TYPE_SOFTWARE` with simply **removing** the `setLayerType(...)` call entirely
  (revert to `LAYER_TYPE_NONE` = default HWUI into the window surface, also above the surface).
- If that still fails (very unlikely), restructure so the darkening does not depend on compositing
  a transparent overlay over the `SurfaceView`: make the `Dialog` full-screen with a `Color.BLACK`
  background and center a square `SurfaceView`, letting the black background be the darkening.
  This changes the look (preview only in the square, no full-frame preview) — treat as last resort.

## Validation
1. Build the APK and install on the **Xiaomi Android 14** device.
2. Open the QR scanner; confirm the side shadows (landscape) / top+bottom shadows (portrait)
   and the white square border are visible and centered in **both orientations**.
3. Rotate the device while the scanner is open (configChanges absorbs rotation) and confirm the
   darkening re-centers correctly (`onSizeChanged` → `invalidate`).
4. Regression check: run on an emulator (any API) and on an Android < 14 device — overlay still
   visible and correctly positioned (software layer must not regress the working cases).
5. Optional root-cause confirmation (skip if confident): with the OLD `LAYER_TYPE_HARDWARE`,
   temporarily set the overlay background to a solid semi-transparent color (e.g. `#80FF0000`);
   it stays invisible on Xiaomi. Switch to `LAYER_TYPE_SOFTWARE` → it appears. Confirms ordering.

## Risks / notes
- `LAYER_TYPE_SOFTWARE` disables HW acceleration for this one view only; irrelevant for a few
  static rects redrawn on size change. No animation/scrolling on this view.
- No impact on the camera capture/decode pipeline (overlay is purely cosmetic).
- `targetSdkVersion="36"` edge-to-edge effects on the Dialog window under system bars are a
  separate, unreported concern on Android 15+ devices — **out of scope** for this fix.

## Open questions
- None blocking. If step 1 unexpectedly fails on Xiaomi, the fallback in step 3 is the next try.
