# Fix F-Droid device-specific startup crash (target 1.11.74)

## Context / root-cause

- F-Droid build SIGABRTs at launch (~2s) for **one** user (linsui); works for the author on all other phones/emulators and on AppGallery/RuStore.
- Crash = Go runtime abort: `signal 6 (SIGABRT) code -6 (SI_TKILL)`, single un-symbolicated frame in `base.apk`. debuggerd cannot unwind Go stacks, so the tombstone carries **no Go error text**.
- Verified the APK is correctly built (rules out packaging/16 KB/toolchain):
  - `.so` stored uncompressed; data offset in APK = `0x8000` (16 KB-aligned); ELF `LOAD` segments aligned `0x4000`; `zipalign -c -P 16 4 crocson-arm64.apk` → `Verification successful`.
  - Crash PC is *inside* running Go code → the native lib loads fine.
- Toolchain and F-Droid recipe are unchanged since the last working F-Droid build (1.11.65): `go 1.25.0`, `ndk r27d`, `fyne package`. So the regression is **source + device-environment**, not build.
- Strongest recoverable cause: **device-locale-dependent panic at startup**. The tombstone TZ is **+0800** (likely a zh locale). The only at-risk startup site is `main.go:390`:
  ```go
  langPrinter = message.NewPrinter(language.MustParse(langCode))
  ```
  where `langCode` = arbitrary device locale set via `lang.SystemLocale().String()` at `main.go:327` and persisted to pref `"lang"`. `MustParse` panics on malformed BCP-47 tags (e.g. underscored `zh_CN`).
- Non-issues confirmed: `settings.go:55` uses a fixed valid list `["en-US","tr-TR","ja-JP","zh-CN","ru-RU"]` (safe); `internal/translations/catalog.go:36` parses the constant `"en-US"` (safe); `webdav.go` init `panic`s decode a hardcoded PEM constant (safe).

## Strategy (decided)

One F-Droid resubmission bundling **fix + capture**:
- Best case (locale panic): crash disappears.
- Worst case (runtime throw): linsui still crashes, but we ruled out the recoverable-panic class and the next step is well-defined.

Capture mechanism (decided): **`defer recover()` only** (minimal risk). Catches recoverable panics in the main goroutine. Does NOT catch Go-runtime `fatal error`/`throw` or panics in other goroutines — documented as the next-iteration fallback.

## Tasks

1. **`main.go:390` — replace `MustParse` with `Parse` + fallback (the likely fix).**
   - ```go
     tag, err := language.Parse(langCode)
     if err != nil {
         langCode = "en"
         a.Preferences().SetString("lang", langCode) // persist sane value so UI works next launch
         tag = language.English
     }
     langPrinter = message.NewPrinter(tag)
     ```
   - Keep behavior identical for valid locales; never panic on device locale.

2. **`main.go` — unconditional startup panic capture (the diagnostic).**
   - Add at the top of the GUI path (right after the `isCLIMode()` early-return block, before `hideConsole()`):
     ```go
     defer func() {
         if r := recover(); r != nil {
             LogD(fmt.Sprintf("PANIC at startup: %v", r))
             LogD(string(debug.Stack()))
             time.Sleep(500 * time.Millisecond) // let logcat flush
             os.Exit(1)
         }
     }()
     ```
   - `fmt`, `os`, `time`, `runtime/debug` are already imported in `main.go`. `LogD` already routes to logcat under tag `croc` on Android (`for_android.go` → JNI `__android_log_write` in `for_android.c`) and is a no-op on other OSes (`for_android0.go`).
   - Leave the existing `CROC_DEBUG` recover block (`main.go:245–259`) untouched; this new recover is additive and unconditional.
   - Tradeoff: caught startup panic now ends in `os.Exit(1)` (clean exit) instead of a SIGABRT tombstone — acceptable; the logcat line is the signal.

## Validation

- Local correctness: extract the parse into a tiny helper and check it never panics for inputs `""`, `"garbage"`, `"zh_CN"`, `"xx_XX"`, `"zh-Hans-CN"`, `"en-US"` → bad inputs yield `language.English`.
- Build `android/arm64`; re-run `zipalign -c -P 16 4 crocson-arm64.apk` → still `Verification successful`; confirm `fyne package` is still reproducible (no non-deterministic output).
- Field (linsui): reinstall, launch, then `adb logcat -d -s croc:V`. Expected outcomes:
  - App starts normally → locale panic was the cause (fixed), **or**
  - A `croc`-tagged `PANIC at startup: …` line + stack appears → real cause identified, fix in next release.

## Risks / open items

- **Residual: runtime throw not captured.** If after this release linsui still crashes with a bare tombstone and **no** `croc` `PANIC at startup` line in logcat, the cause is a Go-runtime `fatal error`/`throw`, not a recoverable panic.
  - Next iteration (out of scope here): install a `dup2(fd 2 → pipe → reader goroutine → LogD)` bridge in an early `init()` to capture runtime throw text, and/or pin/change the Go version in the F-Droid recipe to bisect.
- Optional polish (not required): add an `ERROR`-level C logger in `for_android.c` (e.g. `LogE2` → `ANDROID_LOG_ERROR`) and a Go wrapper so crash lines appear at higher priority than the current `LogD` (DEBUG). Reusing `LogD` is sufficient if linsui captures `-s croc:V`.
