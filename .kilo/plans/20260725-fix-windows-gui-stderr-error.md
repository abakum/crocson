# Fix: `send write /dev/stderr: The handle is invalid.` on Windows (no console)

## Symptom
On Windows, when `crocson.exe` is launched **without a console window** (double‑clicked
from Explorer, not run from `cmd.exe`), pressing the **Send** button on the send page
sets the top status line to:

```
send write /dev/stderr: The handle is invalid.
```

## Root cause (verified)

1. `crocson.exe` (built by `make wsl`, `-ldflags=-s`, no `-H windowsgui`) is a
   **CONSOLE‑subsystem** binary. Confirmed by PE header: `subsystem 3 CONSOLE`.
2. On launch Windows attaches a console, so at package‑var init
   `GUI = syscall.Stdout == 0 && syscall.Stderr == 0` evaluates to **`false`**
   (`main.go:165`) — the handles are valid at that moment.
3. `main()` then calls `hideConsole()` (`main.go:242`). On a double‑click launch
   `GetConsoleProcessList` returns `1`, so `FreeConsole()` runs (`for_windows.go:138`)
   and detaches the console. The cached `os.Stdout` / `os.Stderr` / `os.Stdin` file
   objects now point at a **freed (INVALID_HANDLE) console**.
4. Because `GUI` stayed `false`, `croc.Options` is built with `Quiet: GUI` = `false`
   (`send.go:1363`, `recv.go:1061`). croc therefore does **not** perform its
   `Quiet`‑gated redirect of `os.Stderr` → `os.DevNull`
   (`abakCroc/croc/src/croc/croc.go:214-219`).
5. `client.Send()` writes to `os.Stderr` both directly
   (`fmt.Fprintf(os.Stderr, …)`, croc.go:777) and via the progress bar
   (`progressbar.OptionSetWriter(os.Stderr)`, croc.go:1400/2010/2148/2196).
   Writing to the dead handle fails with Windows error 6
   (`ERROR_INVALID_HANDLE` → Go message `write /dev/stderr: The handle is invalid.`).
6. That write error propagates out as `sendErr`; `send.go:1594-1597` renders it into
   the top status line as `send: <err>`.

So this is **not** a croc bug and **not** specific to the GUI‑subsystem build — it is
crocson freeing the console at runtime while leaving `GUI == false` and the std handles
dangling.

Note: `make windowsgui` / `fyne package` (true GUI‑subsystem) already sets `GUI = true`
at init, so croc’s `Quiet` redirect handles it there; the bug is the **console** build
that frees its console later.

## Fix (crocson only — Windows‑specific)

Make the standard handles safe at the exact moment we destroy the console, and reflect
that state in `GUI`. Edit `for_windows.go` `hideConsole()`:

- After `FreeConsole()` (when `count == 1`), **and** whenever `GUI` is already true
  (genuine GUI‑subsystem build, whose handles are 0/invalid), redirect
  `os.Stdin` / `os.Stdout` / `os.Stderr` to `os.DevNull` (`NUL`, opened `O_RDWR`).
- Set `GUI = true` so the rest of `main` treats the session as GUI:
  - `setOut(GUI)` (`main.go:285-320`) routes logs to the in‑memory viewer only,
    never to the dead `os.Stdout`.
  - `croc.Options{Quiet: GUI, IgnoreStdin: GUI}` (`send.go:1363`, `recv.go:1061`)
    become `true` → croc also redirects its `os.Stderr` to `NUL` and skips stdin.

Condition `freed || GUI` covers all three cases correctly:
- console build, double‑clicked → `freed == true` → redirect, `GUI = true` (fixes bug).
- console build, run from `cmd.exe` → `count > 1`, `freed == false`, `GUI == false`
  → **no redirect**, real console preserved (developer still sees output). ✅ unchanged
- GUI‑subsystem build → `freed == false`, `GUI == true` → redirect dead handles to `NUL`
  (belt‑and‑suspenders; croc’s `Quiet` redirect already handled the reported path).

### Code sketch (`for_windows.go`)

```go
func hideConsole() {
	var buf [2]uint32
	count, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	freed := false
	if count == 1 {
		procFreeConsole.Call()
		freed = true
	}
	// Once the console is gone (freed now, or never existed in a GUI-subsystem
	// build) the cached os.Stdin/Stdout/Stderr point at an INVALID_HANDLE.
	// Any library write to them — notably croc's progress bar and
	// fmt.Fprintf(os.Stderr,…) inside client.Send — fails with
	// "write /dev/stderr: The handle is invalid." and surfaces in the UI.
	// Redirect them to NUL and mark the session as GUI (Quiet/IgnoreStdin,
	// logs to the in-memory viewer).
	if freed || GUI {
		redirectStdioToNull()
	}
}

// redirectStdioToNull points the process standard handles at NUL so writes/reads
// become harmless no-ops instead of failing on a detached console.
func redirectStdioToNull() {
	if f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0); err == nil {
		os.Stdin = f
		os.Stdout = f
		os.Stderr = f
		GUI = true
	}
}
```

Add `"os"` to the imports of `for_windows.go` if not present (it currently imports
`fyne.io/fyne/v2`, `log`, `golang.org/x/sys/windows`, `golang.org/x/sys/windows/registry`,
`fmt`, `net/url`, `os/exec`, `sync/atomic`, `unsafe` — `os` is **not** imported there yet).

`for_windows0.go` (non‑Windows stub) needs no change — `hideConsole`/`clearConsole`
are Windows‑only; on other OSes `GUI`/`Quiet` already behave correctly (mobile forces
`GUI = true` at `main.go:303`; Linux desktop keeps its real stdout).

## Why not change the croc fork

croc already does the right thing when told it is a GUI app (`Quiet` → redirect
`os.Stderr` to `NUL`). The defect is that crocson reports `GUI = false` after freeing
the console. The fix is therefore in crocson. (Optional, out of scope: croc’s internal
logger captures `os.Stderr` at `models/constants.go:60` `init()`, before crocson runs —
its write errors are swallowed by `schollz/logger` and are not the reported bug; not
worth a fork change.)

## Affected boundaries / data flow
- Entry: `main.go:242 hideConsole()` (only non‑mobile desktop path; CLI mode returns at
  `main.go:212` before this).
- Downstream consumers of `GUI`: `send.go:1363`, `send.go:1911` (stdin pipe check),
  `recv.go:1061`, `main.go:304/319 setOut`.
- croc writes that were failing: `croc.go:777` (Send banner), progress bars at
  `croc.go:1400/2010/2148/2196`.

## Validation

1. Build both targets (per `AGENTS.md`):
   ```
   make arm64 wsl
   ```
   `make wsl` must still succeed (compiles the Windows change; no Java involved).
2. Confirm the produced `crocson.exe` still CONSOLE‑subsystem is fine; the change is
   runtime, not link‑time.
3. Manual Windows repro (the actual bug):
   - Double‑click `crocson.exe` (no console window).
   - Pick a file on the Send page, press **Send**.
   - Expect: top line shows the normal “Have them press the Download now” / progress
     text — **no** `write /dev/stderr` error.
4. Regression check (console output preserved): run `crocson.exe` from `cmd.exe`;
   console stays attached, debug log still prints to the console.
5. (If available) Repeat repro with a `make windowsgui` build to confirm no regression
   there either.

## Risks
- Redirecting `os.Stdin` to `NUL` makes reads return EOF. This is the desired “no stdin”
  behavior for a GUI launch and matches `IgnoreStdin: GUI`; CLI/stdin‑pipe mode
  (`send.go:1911`) is gated on `!GUI` and `isCLIMode()` (`main.go:197-209`) returns
  before `hideConsole()`, so piping (`cat file | crocson`) is unaffected.
- Single `*os.File` shared by all three handles is safe: `NUL` is not seekable and
  accepts concurrent writes.
- Setting `GUI = true` flips `setOut` to the in‑memory log viewer for the double‑click
  case — intended (there is no visible console to write to anyway).

## Out of scope
- Modifying the croc fork.
- Changing how the binary’s subsystem is chosen (console vs `windowsgui`).
- Android / iOS / Linux paths (unaffected).
