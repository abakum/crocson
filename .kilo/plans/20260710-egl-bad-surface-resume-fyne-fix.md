# Fix: EGL_BAD_SURFACE on resume-from-Recents (Home → square button)

## Context / Root cause

`failed to swap buffers (EGL_BAD_SURFACE)` is emitted at
`../fyne/internal/driver/mobile/app/android.go:512` — the `mainUI` render loop's
`<-theApp.publish` case calls `eglSwapBuffers(display, C.surface)` on a **stale
surface** that still points at the old `ANativeWindow` Android tore down in
`onStop`.

The surface-recreation logic relies on two flags that are broken:
- `wasDestroyed` is set `true` in the `windowDestroyed` case (`android.go:501`)
  and **never reset to `false`**.
- Therefore the `windowCreated` recreate-path (`android.go:458`, gated
  `if surfaceInitialized && !wasDestroyed`) is **permanently disabled** after the
  first proper window destroy.
- The `windowRedrawNeeded` recreate-path (`android.go:466`) only recreates
  `if C.surface == nil`.

Result: on resume, `onNativeWindowCreated(W1)` arrives with a stale **non-nil**
`C.surface` (bound to dead W0). Neither path recreates it → `eglSwapBuffers` on
the dead surface → `EGL_BAD_SURFACE` repeated for ~30s. Introduced by fork
commit `3f22f0b2d` ("Workaround for old devices/android not calling
onNativeWindowDestroyed"). The reliable repro is Home (round) → Recents (square)
**without** `onDestroy` (fresh `onCreate` works because EGL is rebuilt from zero).

crocson consumes the fork via `go.mod`:
`replace fyne.io/fyne/v2 => github.com/abakum/fyne/v2 v2.0.0-...-afc3082cacc1`.
`../fyne` == that fork (branch `develop`, HEAD `afc3082ca`). The fix lands in
`../fyne`.

## Decision

**Approach: window-pointer tracking (C + Go).** Store the `ANativeWindow*` the
current EGL surface is bound to in a C global; (re)create the surface whenever a
`windowCreated`/`windowRedrawNeeded` event targets a **different** window than
the bound one (or when there is no surface). This removes all dependence on the
fragile `surfaceInitialized`/`wasDestroyed` flags.

Out of scope: `EGL_CONTEXT_LOST` handling (separate, rarer failure; the reported
error is `EGL_BAD_SURFACE`, not context loss). Upstreaming the PR to
fyne.io/fyne is optional follow-up, not required for crocson.

## Changes — all in `../fyne`

### 1. `internal/driver/mobile/app/android.c`

**Add a bound-window global** next to the existing globals (~line 170-172):
```c
EGLDisplay display = NULL;
EGLSurface surface = NULL;
EGLContext context = NULL;
ANativeWindow* boundWindow = NULL;   // window the current surface is bound to
```

**`createEGLSurface`** (~line 181): after `eglCreateWindowSurface` succeeds,
record the window:
```c
	surface = eglCreateWindowSurface(display, config, window, NULL);
	if (surface == EGL_NO_SURFACE) {
		return "EGL create surface failed";
	}
	boundWindow = window;   // <-- ADD
```

**`destroyEGLSurface`** (~line 220): null both the surface and the bound window
after `eglDestroySurface`:
```c
char* destroyEGLSurface() {
	if (!eglDestroySurface(display, surface)) {
		return "EGL destroy surface failed";
	}
	surface = NULL;        // <-- ADD
	boundWindow = NULL;    // <-- ADD
	return NULL;
}
```

**Add helper** (after `destroyEGLSurface`):
```c
// 1 if the EGL surface must be (re)created for `window`
// (no surface yet, or bound to a different window).
int surfaceNeedsRecreate(ANativeWindow* window) {
	return surface == NULL || boundWindow != window;
}
```

### 2. `internal/driver/mobile/app/android.go`

**Remove the flags** (line 450):
```go
	var surfaceInitialized, wasDestroyed bool   // DELETE this line
```

**Add helper** (top-level, same package as `mainUI`):
```go
// ensureSurface guarantees an EGLSurface exists and is bound to w, recreating
// it when it is missing or bound to a different (e.g. already-torn-down) window.
func ensureSurface(w *C.ANativeWindow) error {
	if C.surfaceNeedsRecreate(w) == 0 {
		return nil
	}
	if C.surface != nil {
		if errStr := C.destroyEGLSurface(); errStr != nil {
			return fmt.Errorf("%s (%s)", C.GoString(errStr), eglGetError())
		}
		C.surface = nil
	}
	if errStr := C.createEGLSurface(w); errStr != nil {
		return fmt.Errorf("%s (%s)", C.GoString(errStr), eglGetError())
	}
	DisplayMetrics.WidthPx = int(C.ANativeWindow_getWidth(w))
	DisplayMetrics.HeightPx = int(C.ANativeWindow_getHeight(w))
	return nil
}
```

**Rewrite `windowCreated` case** (~line 458-464):
```go
		case w := <-windowCreated:
			if err := ensureSurface(w); err != nil {
				return err
			}
```

**Rewrite `windowRedrawNeeded` case** (~line 466-476) — replace the
`if C.surface == nil { ... surfaceInitialized = true; DisplayMetrics ... }`
block with:
```go
		case w := <-windowRedrawNeeded:
			if err := ensureSurface(w); err != nil {
				return err
			}
			theApp.sendLifecycle(lifecycle.StageFocused)
			// ... unchanged: widthPx/heightPx/currentSize/events/paint as today
```
(keep the rest of the case — `widthPx`, `currentSize`, the two
`theApp.events.In() <- ...` sends — unchanged).

**`windowDestroyed` case** (~line 494-502): delete the `wasDestroyed = true`
line. Keep `C.surface = nil` (harmless defensive clear; `destroyEGLSurface`
also nulls it in C now):
```go
		case <-windowDestroyed:
			if C.surface != nil {
				if errStr := C.destroyEGLSurface(); errStr != nil {
					return fmt.Errorf("%s (%s)", C.GoString(errStr), eglGetError())
				}
				C.surface = nil
			}
			theApp.sendLifecycle(lifecycle.StageAlive)
```

### Correctness notes
- `onNativeWindowCreated` sends `windowCreated` then forces
  `onNativeWindowRedrawNeeded` **with the same `window` pointer**. `ensureSurface`
  runs for `windowCreated` (creates for W1), then the forced `windowRedrawNeeded`
  sees `boundWindow==W1` → no-op. No double-create.
- All C globals (`surface`, `boundWindow`) are touched only from the single
  `mainUI` goroutine (window callbacks block on unbuffered channels until
  serviced) → no new data race.
- `ANativeWindow*` identity is stable per window instance, so pointer compare in
  `surfaceNeedsRecreate` is safe.

### API-level safety: API28 (the bug) vs API29+ (must not regress)
- **Why only API28/old devices:** fork commit `3f22f0b2d` documents that old
  Android does not reliably deliver `onNativeWindowDestroyed` on background, so
  `C.surface` is left stale → `EGL_BAD_SURFACE`. On API29+ the destroy callback
  fires reliably. Also `GoNativeActivity.java:1265` had (now commented out)
  `SDK_INT <= 28 → finishActivity()` in `onUserLeaveHint`, which used to force a
  fresh `onCreate` on API28 and thus masked the bug.
- **Why API29+ is unaffected by this fix:** the recreate path only triggers when
  `surfaceNeedsRecreate(w)` is true, i.e. `surface==NULL || boundWindow!=w`.
  - API29+ background → `windowDestroyed` nulls surface/boundWindow; resume →
    `windowCreated` sees `surface==NULL` → creates (same as today's forced-redraw
    `C.surface==nil` create). End state identical to current code.
  - **Rotation / config change** (`configChanges`, no window recreate):
    `windowRedrawNeeded` arrives with the SAME window pointer → `boundWindow==w`
    → `surfaceNeedsRecreate` returns 0 → NO recreate, only `currentSize` is
    recomputed below. Rotation behaviour preserved exactly.
  - The stale-surface branch (`boundWindow != w` with non-nil surface) is a
    no-op on API29+ because `boundWindow` is always cleared on destroy; it only
    fires where the destroy callback is missing (the API28 case).
- Net: API29+ takes the same single create it does today; the new
  destroy-and-recreate branch is reachable only on the broken (API28) path.

## Build / validate

> An **API28 emulator is currently running** and is the active adb target — this
> is where the bug reproduces, so the fix is validated end-to-end there.

1. crocson already points at the local fork (`go.mod` replaces → `../fyne`, done
   via `make local`). After editing `../fyne`, run `go mod tidy` in crocson so
   the build picks up the changed sources (no version bump needed while local).
2. Build + install on the running API28 emulator:
   - `make 386` → `fyne package -os android/386 --release --sign`
   - `make adb` → `adb install -r -d crocson.apk`
3. **Confirm the bug reproduces FIRST on the current (unfixed) build** (sanity
   baseline): Home (round) → Recents (square) → restore → see `failed to swap
   buffers (EGL_BAD_SURFACE)` in `make logcat`. (If the base build does NOT
   repro here, stop — the env assumption is wrong and the fix can't be verified
   on this emulator.)
4. Apply the fix (§ Changes), rebuild (`make 386`), reinstall (`make adb`).
5. **Verify the fix:** Home (round) → Recents (square) → restore → expect UI
   redraws within ~1 frame, NO `EGL_BAD_SURFACE` in `make logcat`; `lifecycle:
   resume` followed by clean swaps.
6. Non-regression on the same API28 emulator:
   - Cold launch (force-stop then icon) renders.
   - Rotate while foregrounded renders without flicker / spurious recreate.
   - Background >60s (process kept alive) then restore.
7. Optional API29+/36 non-regression later: `make emulator`
   (`Medium_Phone_API_36.1`) + `make 386 adb`, repeat the rotation and
   Home→Recents checks — expect unchanged behaviour (no recreate on rotation,
   no EGL errors).

## Rollout (after device validation)

1. In `../fyne`: commit on a fix branch off `develop`, push to `abakum/fyne`,
   merge to `develop`.
2. In crocson: switch replaces back from local to the published fork at the new
   commit — `make repo` (resolves `FYNE_FORK@develop` and rewrites the replace
   directive + `go mod tidy`). Confirm `go.mod` no longer points at `../fyne`.
3. Rebuild/release APK.

## Risks / open questions

- **EGL_CONTEXT_LOST** (GPU killed context under memory pressure) is NOT handled
  — would need recreating `context` in `createEGLSurface`/`destroyEGLSurface`
  and re-triggering a full GL re-init. Different symptom; defer unless observed.
- Fork branch strategy (fix branch vs direct to `develop`) — implementer's call;
  recommend a short fix branch + merge to `develop`.
