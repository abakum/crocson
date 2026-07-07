# Fyne 2.7 "fyne.Do[AndWait] called from main goroutine" startup noise — benign, no fix

## Status: DECISION = no code change (investigation-only addendum)

Triggered by linsui/F-Droid logcat work. Companion to `20260707-fdroid-startup-crash-fix.md`.
Conclusion: the two Fyne error lines at Android startup are **cosmetic migration noise**, prefs work correctly. Do not restructure pref init.

## Symptom (Android only)
On cold launch, before the activity lifecycle, logcat shows exactly twice:
```
I/Fyne: *** Error in Fyne call thread, fyne.Do[AndWait] called from main goroutine ***
I/Fyne:   From: fyne.io/fyne/v2@v2.7.4/app/preferences.go:54
```
Desktop does **not** reproduce it.

## Root cause (verified against Fyne 2.7.4 source)
1. `main.go:339-419` runs the "ensure-defaults" pref writes (`SetString(key, StringWithFallback(key, def))`) on the main goroutine **before** `w.ShowAndRun()` (`main.go:426`).
2. First `SetXxx` → `InMemoryPreferences.set` → `fireChange` → change listener (`app/preferences.go:137`) → `p.save()` → `saveToStorage` (writes file synchronously) → `defer p.resetSavedRecently()`.
3. `resetSavedRecently` (`app/preferences.go:49-66`) spawns `go func(){ time.Sleep(100ms); fyne.DoAndWait(...) }`.
4. ~100 ms later that goroutine calls `fyne.DoAndWait` → `Driver().DoFromGoroutine(fn, true)` → `async.EnsureNotMain` (`internal/async/goroutine.go:27`).
5. `EnsureNotMain` errors only if `IsMainGoroutine()` is true. On mobile `IsMainGoroutine()` (`internal/async/goroutine_mobile.go`) = `mainGoroutineID == 0 || goroutineID() == mainGoroutineID`. `SetMainGoroutine()` is called **inside** the `app.Main` callback (`internal/driver/mobile/driver.go:182`), i.e. only after the mobile event loop starts (`ShowAndRun`). Until then `mainGoroutineID == 0`, so **every** goroutine reports as "main" → error fires.
6. Crucially, `EnsureNotMain` still executes the work: on the error path it does `go fn()` (not a drop). So the `DoAndWait` closure (reset `savedRecently`, conditional re-save) **does run** — just on a fresh goroutine.
7. Two errors = first `save()` + the `changedDuringSaving` re-save; once the loop starts and `SetMainGoroutine()` runs, the spawned `resetSavedRecently` goroutine correctly reports non-main → silence. Hence exactly two lines, then clean lifecycle (matches observed logcat).

## Impact: none (functional)
- Pref **reads**: served from in-memory map (`internal/preferences.go`), no `DoAndWait`, always correct.
- Pref **writes**: stored in-memory immediately; persisted to disk by `save()`/`saveToStorage` (synchronous file write) and by the `go fn()` reset path; `forceImmediateSave` also flushes on clean exit.
- No data race: prefs guarded by `sync.RWMutex`; file I/O is goroutine-safe.
- Unrelated to linsui's SIGABRT (app reaches `lifecycle: resume` fine).

Residual theoretical-only risk: a future Fyne version could change `EnsureNotMain` to drop `fn` instead of `go fn()`, or flip the `MigratedToFyneDo()` build flag. Not actionable now.

## Decision (confirmed with user)
**Do not restructure pref init.** The ~40-line ensure-defaults block touches many keys; rewriting it for cosmetic noise is unjustified risk. Leave `main.go:339-419` as-is. The linsui crash goal is covered by the already-shipped capture mechanism + `MustParse`→`Parse` fallback (`20260707-fdroid-startup-crash-fix.md`).

## If ever revisited (out of scope; recorded to prevent re-investigation)
Cleanest elimination without touching read paths:
- Keep all **reads** (`StringWithFallback`/`BoolWithFallback`/`Int`/`String`) before `ShowAndRun` — reads never trigger `save()`, so they are safe.
- Move only the **writes** (`SetString`/`SetBool`/`SetInt` ensure-default pairs) into `a.Lifecycle().SetOnStarted(func(){ ... })` (pattern already used in `privacy.go:27` with the same rationale). By the time `OnStarted` fires, the loop is running and `mainGoroutineID` is set, so the spawned `resetSavedRecently` goroutine reports non-main → no error.
- Do **not** wrap writes in `fyne.Do` from a pre-loop goroutine: on mobile that races `mainGoroutineID==0` and re-triggers the error.

## Validation
- Re-read logcat: confirm exactly 2× `preferences.go:54` error at startup, no `croc`-tagged `PANIC at startup`, app reaches `lifecycle: resume`. Already satisfied by the provided protocol.
- If the noise is ever to be removed, after the OnStarted refactor re-capture `adb logcat -s Fyne:V` and confirm zero `preferences.go:54` lines on a fresh install.
