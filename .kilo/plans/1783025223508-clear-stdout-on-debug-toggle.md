# Plan: Clear system console (stdout) on debug toggle

## Context
When the `debug` checkbox on the log/protocol tab is toggled, the GUI log is already
cleared (log.go:295-298 resets `logOutput.buf`, `lastLines`, `segments` and refreshes).
The logger also writes to the system console (`os.Stdout`) on desktop when run from a
terminal (main.go:282, `io.MultiWriter(os.Stdout, &logOutput)`). Goal: clear that
system console too, so both log surfaces reset together.

Routing recap (main.go:276-310 `setOut`):
- Desktop, `GUI=false` (run from terminal): `io.MultiWriter(os.Stdout, &logOutput)` — stdout active.
- Mobile / `GUI=true`: only `&logOutput` — stdout not written.
- `crocDebug` on: `io.MultiWriter(crocdebuglog, &logOutput)` — file, out of scope here.

## Decisions (resolved)
1. **Target:** desktop terminal `os.Stdout`. The debug log file (`crocdebuglog.txt`) and
   Android logcat / iOS os_log are **out of scope** (logcat/OS logs are not clearable
   from the app).
2. **Trigger:** clear on **both** enable and disable of `debug` — mirrors the existing
   GUI clear (log.go:295-298).
3. **Non-Windows (`!windows`, incl. darwin/linux/android/ios):** ANSI escape
   `\x1b[2J\x1b[3J\x1b[H` (screen + scrollback + cursor home), gated by a TTY check via
   `os.Stdout.Stat()` + `os.ModeCharDevice` (same idiom as `isPiped()`, main.go:189).
   Mobile naturally no-ops (stdout not a char device / not written to).
4. **Windows:** native Console API (kernel32 via already-imported `golang.org/x/sys/windows`),
   not VT escapes. Works on all Windows versions, no risk of printing raw `\x1b[2J` garbage.
5. **File placement — no new files.** Follow repo convention `for_X.go` / `for_X0.go`:
   - `for_windows0.go` (`//go:build !windows`) → ANSI `clearConsole()`.
   - `for_windows.go` (`//go:build windows`) → native-API `clearConsole()`.
   - `log.go` (no build tag) → call site in the `debugCheck` callback.

## Task list

### 1. Add `clearConsole()` to `for_windows0.go` (`//go:build !windows`)
Current file is a 7-line stub importing only `net/url` (defines `netUse`).
- Add imports `fmt`, `os`.
- Implement:
  ```go
  func clearConsole() {
      fi, err := os.Stdout.Stat()
      if err != nil {
          return
      }
      if fi.Mode()&os.ModeCharDevice == 0 { // not a real terminal (pipe/file/redirect)
          return
      }
      fmt.Fprint(os.Stdout, "\x1b[2J\x1b[3J\x1b[H") // clear screen + scrollback + home
  }
  ```

### 2. Add `clearConsole()` to `for_windows.go` (`//go:build windows`)
Already imports `golang.org/x/sys/windows` (for_windows.go:27) and loads kernel32 procs.
- Implement native Console-API clear:
  - `h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)` — on err, return.
  - `var info windows.ConsoleScreenBufferInfo; err = windows.GetConsoleScreenBufferInfo(h, &info)` — on err (no console, e.g. windowsgui build), return.
  - Compute total cells = `uint32(info.Size.X) * uint32(info.Size.Y)`; home coord `windows.Coord{X: 0, Y: 0}`.
  - `windows.FillConsoleOutputCharacter(h, ' ', total, home, &written)`.
  - `windows.FillConsoleOutputAttribute(h, info.Attributes, total, home, &written)` (reset colors).
  - `windows.SetConsoleCursorPosition(h, home)`.
  - Ignore non-fatal errors; the goal is best-effort clear.
- Note: uses `windows.GetStdHandle` (takes `STD_OUTPUT_HANDLE`); verify exact symbol names
  against the installed `golang.org/x/sys/windows` v0.43.0 during implementation.

### 3. Wire into the debug checkbox callback (log.go)
In `debugCheck := widget.NewCheck("debug", func(debug bool) { ... })` (log.go:281-299),
add `clearConsole()` next to the existing GUI clear block (log.go:295-298). It runs on
both toggle directions (the callback already fires for enable and disable).
- `log.go` needs no new imports for the call itself (function lives in the `for_windows*` files).

## Risks / edge cases
- Clears the **entire** terminal including scrollback — intended. Only fires on a real
  TTY / console, so GUI apps, redirected output, and `crocDebug`-file mode are unaffected.
- Windows `windowsgui` build (Makefile:177, `-H windowsgui`): no console allocated →
  `GUI=true`, stdout not written; even if `clearConsole` runs,
  `GetConsoleScreenBufferInfo` fails → safe no-op.
- Windows console build (`make windows`, Makefile:183) launched from cmd/PowerShell/Windows
  Terminal: native clear works on all versions (no VT dependency). `hideConsole()`
  (for_windows.go:130) hides the console in normal launches; clearing still works for
  dev runs where the console is visible.
- Classic `conhost` may not support `\x1b[3J` scrollback clear on the `!windows` path is
  N/A there; on Windows the native fill clears the whole buffer (scrollback included).
- Do not print escape bytes into redirected streams — guarded by the char-device check on
  `!windows` and by `GetConsoleScreenBufferInfo` failure on Windows.

## Validation
- `go build ./...` and `go vet ./...`.
- Cross-compile to confirm the build-tag split compiles on every target:
  `GOOS=darwin go build`, `GOOS=linux go build`, `GOOS=windows go build`.
- macOS: run `./crocson` from Terminal → toggle `debug` on/off → terminal clears together
  with the GUI log; toggle both ways to confirm symmetry.
- Redirected: `./crocson > out.txt` → toggle `debug` → confirm **no** escape bytes
  (`\x1b`) written into `out.txt`.
- Windows (if available): console build from cmd/Windows Terminal → toggle `debug` →
  console clears; `app.exe > out.txt` → nothing written.
- Mobile (android/ios) build compiles and `clearConsole` is a no-op there (char-device
  check / no stdout).

## Out of scope
- Clearing `crocdebuglog.txt` (feasible via `Truncate(0)`, but user scoped to stdout only).
- Clearing Android logcat / iOS os_log (not possible from within the app).
- Enabling ANSI VT via `SetConsoleMode` (superseded by native Console API approach).
