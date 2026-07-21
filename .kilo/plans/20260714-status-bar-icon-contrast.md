# Invisible Android status bar icons + Android-10 (API 29) dark-mode-on-return regression — mode-based runtime adaptation

## Context / root cause
- Status bar icons were invisible on light/system Fyne themes (visible on dark). Version-independent.
- Custom `fyne package` links `aapt2 ... -I android-36/android.jar --auto-add-overlay -R
  compiled_res.zip`, baking API-36 edge-to-edge / transparent status bar into the APK. The Fyne
  fork never sets `windowLightStatusBar` → default white icons are invisible on a light Fyne bg.
- `windowLightStatusBar` has no `auto`; `statusBarColor` is ignored on edge-to-edge. AppCompat /
  Material `DayNight` is unavailable (no AndroidX).

### Android 10 (API 29) regression — the focus-return bug
- **Symptom:** toggle system dark mode via the quick-settings shade → on Android 14 the app
  (content theme **and** status bar) updates immediately; on Android 10 (API 29) it does **not**
  change when returning to the app.
- **Cause:** `updateTheme(config)` is the single chokepoint that drives *both* the Fyne content
  theme (via the native `setDarkMode(dark)` call — Java→Go, declared `GoNativeActivity.java:75`)
  **and** the status bar (`systemDark` + `applyStatusBarIcons()`). It runs only from `onCreate`
  (`:1559`) and `onConfigurationChanged` (`:1694`). On API 29 a `uiMode` change toggled from the
  shade is not delivered as `onConfigurationChanged` before/at resume — it is dropped or deferred
  indefinitely — so `updateTheme` never runs and both the content theme and status bar stay stale.
  Android 14 delivers the config change promptly → no issue.
- **Diagnostic corroboration:** on the same Android 10 device, toggling *color inversion*
  (accessibility) from the shade updates everything **immediately**, while toggling *system dark
  mode* does not. This isolates the bug precisely — color inversion is a compositor/display
  transformation (no `Configuration`/`uiMode` change, no app involvement, no config event → applies
  on every Android version); `UI_MODE_NIGHT` is a `Configuration` change the app must react to via
  `onConfigurationChanged` → `updateTheme`. Rendering itself is fine — only the `uiMode`
  config-reaction path is broken on API 29, which is exactly what the `onResume` reconciler
  re-reads. (Color inversion needs no app handling → **out of scope**.)
- Consequence: status-bar-only is **not** a coherent fix — content and status bar share the same
  chokepoint. Fixing one means re-running `updateTheme` (fixes both).

## Decision (mode-based + resume reconcile)
- Go pushes only the theme MODE to Java (system/light/dark) from `setThemeColor`. Java applies the
  status bar on its own lifecycle hooks (`onCreate` + `onConfigurationChanged` via the existing
  `updateTheme`). Java owns system-theme detection end to end; no Fyne-lifecycle coupling.
- **NEW (this amendment):** add a reconciler in Java `onResume` that re-reads
  `Configuration.uiMode` and, if it differs from the cached `systemDark`, re-runs the existing
  `updateTheme`. This closes the API-29 gap on every foreground return; guarded by a change check
  so a normal resume (theme unchanged) is a no-op.
- Trade-off: status bar color is fixed white/black (not the exact Fyne bg); cosmetic, ignored on
  edge-to-edge. `themes.xml` (`values/` + `values-night/`) stays as the static baseline.

## Current state (as of commit `994b3c4` + working tree) — for the implementer
The mode-based refactor is **already implemented**; the only remaining work is the `onResume`
reconciler (this plan's change).
- ✅ Go mode-based: `theme.go` (`themeMode` + `setAppThemeMode(themeMode(...))` hook in
  `setThemeColor`); `for_android.go` / `for_android0.go` `setAppThemeMode` bridge. The earlier
  color-based helpers (`applyStatusBar` / `setStatusBarBackground` / `colorToARGB`) were removed.
- ✅ Java mode-based: `appThemeMode` / `systemDark` fields, `setAppThemeMode`,
  `applyStatusBarIcons`, `updateTheme` hook (`onCreate:1559` + `onConfigurationChanged:1694`).
- ✅ `res/values/themes.xml` + `res/values-night/themes.xml`: style **`DeviceDefault`** (NOT
  `Light` as an earlier draft of this plan said) —
  `values/`: parent `@android:style/Theme.DeviceDefault.Light.NoActionBar`,
  `windowLightStatusBar=true`, `statusBarColor=@android:color/white`;
  `values-night/`: parent `@android:style/Theme.DeviceDefault.NoActionBar`,
  `windowLightStatusBar=false`, `statusBarColor=@android:color/black`.
  `AndroidManifest.xml:19` uses `android:theme="@style/DeviceDefault"` and declares
  `configChanges="...|uiMode"` (`:16`); `targetSdkVersion=36` (not the cause).
- ❌ MISSING: `onResume` reconciler — the API-29 fix below.

## Change to make

### Java — `onResume` reconciler (`GoNativeActivity.java:1735`)
Replace the no-op `onResume` body with a guarded reconcile:

```java
@Override
protected void onResume() {
    super.onResume();
    Log.d(TAG, "Java: onResume");
    lifecycleEvent("resume");
    reconcileSystemTheme();
}
```

Add:

```java
/**
 * Re-applies the system dark state on foreground return. Fixes API 29 (Android 10), where a
 * uiMode change toggled from the quick-settings shade is not delivered as
 * onConfigurationChanged before/at resume, leaving the Fyne content theme and the status bar
 * stale. No-op when the system theme has not changed.
 */
private void reconcileSystemTheme() {
    Configuration cfg = getResources().getConfiguration();
    boolean dark = (cfg.uiMode & Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES;
    if (dark != systemDark) {
        updateTheme(cfg); // re-runs setDarkMode (native -> Fyne content) + applyStatusBarIcons
    }
}
```

Why this is minimal and correct:
- Reuses the existing `updateTheme(cfg)` chokepoint — no new state, no Fyne-lifecycle coupling.
  `updateTheme` calls `setDarkMode(dark)` (native → Fyne re-renders content), sets
  `systemDark = dark`, then `applyStatusBarIcons()`. One call fixes content **and** status bar.
- The `dark != systemDark` guard makes every normal resume a no-op (no needless Fyne refresh);
  it only fires when a system uiMode change was missed.
- Baseline is correct: `onCreate:1559` already initializes `systemDark` via `updateTheme`, so the
  first `onResume` compares against the real value (no spurious refresh on first launch).
- `onResume` is the single correct hook — it runs after `onStart` (first launch) and after
  `onRestart` (return from background); no need to duplicate in `onStart`/`onRestart`.
- Threading: `onResume` runs on the UI thread, same as `onCreate`/`onConfigurationChanged`.
  `setDarkMode` JNI is synchronous (same as today); `applyStatusBarIcons` already posts
  `runOnUiThread` (fine when called from the UI thread). No new threading concerns.

## Validation
1. `go vet ./...` (linux, uses `for_android0.go` stub) — exit 0. (Unchanged; this change is Java-only.)
2. `javac -cp <android-36>/android.jar GoNativeActivity.java` — exit 0 (deprecation note for the
   API<30 flags is expected).
3. aapt2 compile `res/` + `aapt2 link -I <android-36>/android.jar --auto-add-overlay -R compiled.zip
   --manifest AndroidManifest.xml` — no "resource not found".
4. `make arm64` (with `go` + `fyne` on PATH) — APK builds.
5. Install on Xiaomi Android 14 + an Android 10 (API 29) device: for each theme
   (system/light/grey/dark/black) icons visible (dark on light bg, light on dark bg).
6. Switch theme at runtime → updates immediately.
7. **API-29 regression (primary target):** on the Android 10 device, open the app, pull the
   quick-settings shade, toggle system dark → return to the app → **both** the Fyne content theme
   and the status bar icons update correctly. Repeat light↔dark both directions. Then, with the
   system theme unchanged, background/return → confirm no flicker / spurious refresh (guard works).
8. **Android 14:** same shade-toggle test → still updates immediately (no regression); confirm the
   guard is a no-op when theme unchanged.
9. Regression: no black bottom nav bar (only status bar touched); Fyne content below the status bar
   (`insetsChanged`/`updateLayout`).

## Risks / notes
- `setStatusBarColor` is a no-op on edge-to-edge (API 35+) — by design; `windowLightStatusBar`
  (via `applyStatusBarIcons`) is the operative fix.
- At Java `onCreate`, `appThemeMode` defaults to 0 (system) until Go pushes the saved mode
  (`setThemeColor` at startup, before `ShowAndRun`); brief flash possible for a saved forced
  dark/black theme, but the window is not yet visible.
- `reconcileSystemTheme` reuses `updateTheme`; if (defensively) `setDarkMode` is invoked with the
  same value as before, Fyne treats it as a refresh — harmless. The guard prevents this in normal
  operation.
- Does not change the `-I android-36` build; adapts at runtime.

## Out of scope
- **Color inversion (accessibility):** compositor/display transformation, not a `Configuration` or
  `uiMode` change. Applies to rendered pixels system-wide with no app involvement and no config
  event → works on every Android version already (confirmed on API 29). Nothing to do here.
- Patching the Fyne fork to set status bar appearance / detect uiMode for all apps.
- AppCompat/Material `DayNight` (no AndroidX; blind to Fyne forced themes).
- Changing the `-I android-36` build to `android-34` or binres.

## Supersedes
- The earlier color-based version of this plan; `.kilo/plans/20260714-android-status-bar-missing.md`
  (wrong edge-to-edge / targetSdk 34 hypothesis).
