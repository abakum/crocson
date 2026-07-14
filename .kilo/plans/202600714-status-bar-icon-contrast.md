# Fix invisible Android status bar icons (light Fyne theme) — mode-based runtime adaptation

## Context / root cause
- Symptom: status bar icons invisible on light/system Fyne themes; visible on dark.
  Version-independent (Android 9 Yandex + Android 14 Xiaomi).
- Custom `fyne package` links `aapt2 ... -I android-36/android.jar --auto-add-overlay -R
  compiled_res.zip`, baking API-36 edge-to-edge / transparent status bar into the APK (all
  devices). The Fyne fork never sets `windowLightStatusBar` → default white icons are
  invisible on a light Fyne bg. The binres build (no `-I`) has no bug.
- `windowLightStatusBar` has no `auto`; `statusBarColor` has no `background` keyword and is
  ignored on edge-to-edge. AppCompat/Material `DayNight` + `setDefaultNightMode` are
  unavailable (no AndroidX; Fyne Java compiles against `android.jar` only) and blind to
  Fyne's forced themes.

## Decision (mode-based)
- Go pushes only the theme MODE to Java (system/light/dark) from `setThemeColor`. Java
  applies the status bar on its own lifecycle hooks (`onCreate` + `onConfigurationChanged`
  via the existing `updateTheme`). This covers the foreground system-dark-flip case (via
  `onConfigurationChanged`, since `configChanges` includes `uiMode`) that a Go-applied /
  color-based design misses, and removes Fyne lifecycle coupling.
- Trade-off: status bar color is fixed white/black (not the exact Fyne bg); cosmetic,
  ignored on edge-to-edge. `themes.xml` (`values/` + `values-night/`) stays as the static
  baseline.

## Changes

### 1. `res/values/themes.xml` + `res/values-night/themes.xml` — static baseline
- `values/themes.xml` style `Light`, parent `@android:style/Theme.Light.NoTitleBar`:
  `windowLightStatusBar=true` (dark icons) + `statusBarColor=@android:color/white`.
- `values-night/themes.xml` style `Light` (same name; auto-resolves under system dark):
  `windowLightStatusBar=false` (light icons) + `statusBarColor=@android:color/black`.
- Correct static baseline for the "system" theme before Go runs; runtime overrides forced
  themes.

### 2. `AndroidManifest.xml`
- `android:theme="@style/Light"` (keep); `android:targetSdkVersion="36"` (keep; not the
  cause).

### 3. Go — push mode from `setThemeColor`
- `theme.go`: in `setThemeColor(themeName)`, after the switch, call
  `setAppThemeMode(themeMode(themeName))`. Add:
  ```go
  func themeMode(name string) int32 {
      switch name {
      case "system": return 0
      case "light":  return 1
      default:       return 2 // grey, dark, black
      }
  }
  ```
- `for_android.go` (`//go:build android`):
  ```go
  func setAppThemeMode(mode int32) {
      if err := callVoidInt("setAppThemeMode", mode); err != nil {
          log.Errorf("setAppThemeMode: %v", err)
      }
  }
  ```
  Reuses existing `callVoidInt` + its C bridge (`for_android.c`/`for_android.h`); Java
  method signature is `(I)V`.
- `for_android0.go` (`//go:build !android`): `func setAppThemeMode(mode int32) {}`.
- Triggers are covered via the `setThemeColor` callers: startup (`main.go:408`) + user
  theme change (`settings.go:65`). No Fyne lifecycle hooks.

### 4. Java — apply on lifecycle (`GoNativeActivity.java`)
- Fields: `private static int appThemeMode = 0;` (0=system, 1=light, 2=dark) and
  `private static boolean systemDark = false;`.
- `static void setAppThemeMode(int mode)` → store `appThemeMode = mode;` + call
  `applyStatusBarIcons()`.
- In `updateTheme(Configuration config)` (already runs on `onCreate` +
  `onConfigurationChanged`), after `setDarkMode(dark)`: `systemDark = dark;
  applyStatusBarIcons();`.
- `static void applyStatusBarIcons()`:
  - `lightBg = (appThemeMode == 1) || (appThemeMode == 0 && !systemDark);`
  - `runOnUiThread`: `window.setStatusBarColor(lightBg ? Color.WHITE : Color.BLACK)`
    (cosmetic; no-op on edge-to-edge); then set `windowLightStatusBar`:
    - API ≥ R: `WindowInsetsController.setSystemBarsAppearance(lightBg ?
      APPEARANCE_LIGHT_STATUS_BARS : 0, APPEARANCE_LIGHT_STATUS_BARS)`.
    - else: `View.setSystemUiVisibility` toggle `SYSTEM_UI_FLAG_LIGHT_STATUS_BAR`.
- Import `android.view.WindowInsetsController` (API 30 class; only referenced inside the
  `SDK_INT >= R` branch). `Color`/`Window`/`View`/`Build` already imported.

## Working-tree state (IMPORTANT for the implementer)
The repo is mid-refactor and INCONSISTENT:
- Java (`GoNativeActivity.java`) is ALREADY mode-based (`setAppThemeMode` /
  `applyStatusBarIcons` / fields / `updateTheme` hook). Keep; verify it compiles.
- Go is STILL color-based from a first attempt: `applyStatusBar` / `setStatusBarBackground`
  / `colorToARGB` in `theme.go`; `setStatusBarBackground` bridge in `for_android.go` /
  `for_android0.go`; `applyStatusBar(a)` calls in `main.go` (after `SetTheme`),
  `settings.go` (themeSelect), `send.go` (`OnEnteredForeground`).
- Reconcile Go to section 3: replace the color-based helpers with `themeMode` + the
  `setAppThemeMode` call in `setThemeColor`; replace the `setStatusBarBackground` bridge
  (both files) with `setAppThemeMode`; DELETE the `applyStatusBar(a)` trigger calls in
  `main.go`/`settings.go`/`send.go`. As-is the build passes but the feature is broken at
  runtime (Go calls `setStatusBarBackground`, which Java no longer defines; Java's
  `setAppThemeMode` is never called).

## Validation
1. `go vet ./...` (linux, uses `for_android0.go` stub) — exit 0.
2. `javac -cp <android-36>/android.jar GoNativeActivity.java` — exit 0 (deprecation note
   for the API<30 flags is expected).
3. aapt2 compile `res/` + `aapt2 link -I <android-36>/android.jar --auto-add-overlay -R
   compiled.zip --manifest AndroidManifest.xml` — no "resource not found".
4. `make arm64` (with `go` + `fyne` on PATH) — APK builds.
5. Install on Xiaomi Android 14 + Yandex Android 9: for each theme
   (system/light/grey/dark/black) icons visible (dark on light bg, light on dark bg).
6. Switch theme at runtime → updates immediately.
7. Toggle system dark mode (incl. via quick-settings while foreground) →
   `onConfigurationChanged` → icons correct (system theme; forced themes unaffected).
8. Regression: no black bottom nav bar (only status bar touched); Fyne content below the
   status bar (`insetsChanged`/`updateLayout`).

## Risks / notes
- `setStatusBarColor` is a no-op on edge-to-edge (API 35+) — by design; `windowLightStatusBar`
  is the operative fix.
- At Java `onCreate`, `appThemeMode` defaults to 0 (system) until Go pushes the saved mode
  (from `setThemeColor` at `main.go:408`, before `ShowAndRun`); for a saved forced dark/black
  theme there may be a brief flash until the push — the window is not yet visible, so
  generally not seen.
- If `callVoidInt` runs before the JNI bridge is ready, it errors + logs; `updateTheme`
  re-applies on the next config change using the stored mode (once Go has pushed it).
- Does not change the `-I android-36` build; adapts at runtime.

## Out of scope
- Patching the Fyne fork to set status bar appearance for all apps.
- AppCompat/Material `DayNight` (no AndroidX; blind to Fyne forced themes).
- Changing the `-I android-36` build to `android-34` or binres.

## Supersedes
- The earlier color-based version of this plan; `.kilo/plans/20260714-android-status-bar-missing.md`
  (wrong edge-to-edge / targetSdk 34 hypothesis).
