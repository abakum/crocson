# Plan: Fix Pending Saves Propagation (Android pre-API 29)

Follow-up to `.kilo/plans/20260721-android-storage-permission-pending-saves.md`. The original
feature was implemented but is effectively **dead code** on the recv/log paths: the
`ErrPermissionPending` sentinel never reaches the callers, so `AddPendingSave` is never invoked
and the pending queue stays empty. This plan makes the feature functional and adds robustness,
edge-case handling, and completion UX.

## Root Causes

1. **Java `null` collapses into `errJNI`.** When `createFileInDownloads` returns `null` on the
   permission-pending branch, C `callStringString2` (`for_android.c:178-188`) returns `NULL`,
   which Go interprets as `errJNI` (`for_android.go:164-165`). The `result == ""` check in
   `CreateFileInDownloads` (`for_android.go:1005`) is therefore unreachable and
   `ErrPermissionPending` is never returned.
2. **`ChildDownload` severs the error chain.** `for_android.go:1029` wraps with `%v`, so even if
   the sentinel reached it, `errors.Is(err, ErrPermissionPending)` would be false.
3. **Robustness gaps:** `processPendingSaves`/`clearPendingSaves` are `var func()` assigned inside
   `recvTabItem` (`recv.go:33-34`) but invoked from the `sendTabItem` lifecycle goroutine
   (`send.go:1641-1647`) with no nil-guard; `processPendingSaves` runs synchronously in that single
   lifecycle goroutine, blocking `resume`/`stop`/`qr*` events; zip-failure branch
   (`recv.go:1494-1498`) does not delete the pending entry; `permissionRequested` is `static`
   (`GoNativeActivity.java:99`) and can stay `true` forever if `onRequestPermissionsResult` never
   fires (dialog dismissed abnormally); no success feedback.

## Design Decisions

- **Java pending signal:** return `""` (empty string) from the pending branch instead of `null`.
  Verified unambiguous: `createFileInDownloadsModern`/`Legacy` only ever return a non-empty URI or
  `null` (on exception). Thus C propagates `""` → Go reaches the `result == ""` branch → returns
  `ErrPermissionPending`. Exception path keeps returning `null` → `errJNI` (unchanged).
- **Error wrapping:** `%v` → `%w` in `ChildDownload` so `errors.Is` traverses to the sentinel.
- **Concurrency:** `processPendingSaves` runs in its own goroutine (spawned at the call site in
  `send.go`) so it does not block the lifecycle goroutine. Per-entry copies are already async with
  `fyne.Do` UI callbacks; `removeEntry` is already called from copy goroutines in the original
  `filesSave` path, so the safety profile is unchanged.
- **Completion detection:** snapshot keys at start, count them, atomically decrement on every
  terminal outcome (sync failure, copy success, copy error, zip error); when the counter reaches 0,
  update `topline` + show toast via `fyne.Do`.
- **`permissionRequested` reset:** in `onCreate` (covers killed-dialog → relaunch; the flag is
  process-`static`).

## Changes

### 1. Java — `GoNativeActivity.java`

- **`createFileInDownloads` (~line 1170):** change the pending `return null;` to `return "";`.
  Keep the `catch` block's `return null;` (line ~1176) unchanged.
- **`onCreate` (~line 1625):** add `permissionRequested = false;` (reset any stuck flag on activity
  (re)creation).

### 2. Go — `for_android.go`

- **`ChildDownload` (line 1029):** `fmt.Errorf("createFileInDownloads failed: %v", err)` → use `%w`.
  (Leave the `parse URI failed` `%v` at line 1035 as is — not a sentinel path.)

### 3. Go — `recv.go`

- **Rework `processPendingSaves` (lines 1434-1509):**
  1. Early-return if not Android.
  2. Build a snapshot `[]*PendingSave` from `pendingSaves.Range`; if empty, return immediately
     (no toast).
  3. `var remaining int32 = int32(len(snapshot))`.
  4. Define `finishOne()` = `if atomic.AddInt32(&remaining, -1) == 0 { fyne.Do(func(){ topline.SetText(lp("Saved all files to") + " Download"); NewToast(w, lp("Saved all files to") + " Download").Show() }) }`.
  5. For each `ps` in snapshot:
     - `CreateFileInDownloads(ps.Dest, "")` → on error: `pendingSaves.Delete(ps.Src)`, toast
       (if `ps.W != nil`), `finishOne()`, `continue`.
     - `storage.ParseURI` → on error: delete, `finishOne()`, `continue`.
     - `storage.Writer` → on error: delete, toast, `finishOne()`, `continue`.
     - `copyFrom(src)` success callback: `removeEntry(src, ps.FE, true)`;
       `pendingSaves.Delete(ps.Src)`; `finishOne()`.
     - `copyFrom` error callback: toast; `finishOne()` (do NOT remove entry — leave it for
       retry/visibility, matching existing error behavior).
     - `isLinkDir(ps.Src)` zip path: on zip error, **`pendingSaves.Delete(ps.Src)`** (fixes the
       current leak) + `removeEntry(pathZip, ps.FE, true)` + `finishOne()`; on zip success proceed
       to `copyFrom(pathZip)` (its terminal callback already calls `finishOne()`).
  6. Ensure exactly one `finishOne()` per snapshot entry across all branches.
- Leave `clearPendingSaves` (lines 1511-1520) unchanged.

### 4. Go — `send.go`

- **Lifecycle cases (lines 1641-1647):**
  ```go
  case "storagePermissionGranted":
      if processPendingSaves != nil {
          go processPendingSaves()
      }
  case "storagePermissionDenied":
      if clearPendingSaves != nil {
          clearPendingSaves()
      }
      fyne.Do(func() {
          NewToast(w, lp("Storage permission required")).Show()
      })
  ```

### 5. No `for_android0.go` change

`ErrPermissionPending`, `PendingSave`, `pendingSaves`, `processPendingSaves`, `clearPendingSaves`
all live in shared `recv.go`; `AddPendingSave` already has its non-Android stub. Nothing to add.

## Validation

1. `make arm64 wsl` — both targets compile (per `AGENTS.md`).
2. **Pre-API 29 device:** mass save → permission dialog → **grant** → all queued files save; entries
   leave the list; `topline` + toast confirm completion.
3. **Pre-API 29 device:** mass save → dialog → **deny** → toast "Storage permission required"; no
   files saved; queue cleared.
4. **Post-API 29 device:** mass save works without a permission request (existing behavior
   unchanged).
5. **Non-Android build:** `make wsl` succeeds; runtime unaffected (stubs guard `AddPendingSave`).
6. **Edge case:** trigger save → permission dialog → kill app/force-stop during dialog → relaunch →
   `permissionRequested` is reset → next save re-requests permission (no stuck silent-null).
7. **Counter correctness:** mixed batch (some files, one directory) on pre-29 grant → completion
   toast fires exactly once after the last terminal copy.

## Risks / Open Notes

- **Concurrent save re-entry:** if the user triggers another mass save while `processPendingSaves`
  is mid-flight, both touch `fileentries`/`pendingSaves`. `sync.Map` is safe; `removeEntry` is
  already used from copy goroutines. Acceptable; not adding a global lock to avoid serializing
  normal saves.
- **Counter precision relies on exactly one `finishOne()` per snapshot entry** — review branch
  coverage carefully during implementation (especially the `isLinkDir` zip-success → copy-success
  chain, which must still call `finishOne()` exactly once).
- Line numbers above are approximate (drift since the original plan); locate each site by symbol
  name.
