# Plan: Android Storage Permission Pending Saves

## Context

On Android pre-API 29 (SDK < 29), mass file saves from recv tab fail because `CreateFileInDownloads` returns `null` immediately after requesting storage permission via `requestPermissions`. The async permission response (`onRequestPermissionsResult`) is never awaited, so all files in the loop get errors even if the user grants permission later.

## Goal

Enable pending save operations that are executed after storage permission is granted on Android pre-API 29, preventing data loss when user grants permission.

## Design Decisions

### Data Structure (Go)
- **Location:** Go side (`for_android.go`)
- **Type:** `sync.Map` for concurrent access
- **Key:** `src` string (source file path)
- **Value:** `*PendingSave` struct with:
  - `Src` string — source file path
  - `Dest` string — destination filename
  - `LU` fyne.ListableURI — target folder (nil for Downloads)
  - `FE` *fyne.Container — UI container (for progress bar)
  - `W` fyne.Window — window context for toasts

### Permission Coordination
- **Java side:** Static boolean flag `permissionRequested` in `GoNativeActivity.java`
- **Flow:**
  1. First file without permission → `checkSelfPermission` → `requestPermissions` + set flag = true
  2. Subsequent files → `checkSelfPermission` sees already requested → return null without re-requesting
  3. `onRequestPermissionsResult` → lifecycle events → Go processes/clears pending
  4. Flag reset in Java (optional, auto-resets with next request)

### Error Signaling
- **Constant:** `var ErrPermissionPending = errors.New("permission request pending")`
- **Usage:** `CreateFileInDownloads` returns `("", ErrPermissionPending)` when permission not granted
- **Detection:** `errors.Is(err, ErrPermissionPending)` in calling code

### Lifecycle Events
Add two new lifecycle events in Java (`onRequestPermissionsResult`):
- `lifecycleEvent("storagePermissionGranted")` → Go processes pending saves
- `lifecycleEvent("storagePermissionDenied")` → Go clears pending + shows error toast

### Handler Placement
Add cases in existing `lifecycleFromJava` switch in `send.go:1618`:
```go
case "storagePermissionGranted":
    processPendingSaves()
case "storagePermissionDenied":
    clearPendingSaves()
    fyne.Do(func() {
        NewToast(w, lp("Storage permission required")).Show()
    })
```

### Granted Processing
Recursive call to same functions used in original save:
- Iterate `pendingSaves.Range(...)`
- For each entry: call `CreateFileInDownloads` → copy → cleanup
- Delete from sync.Map after processing

### Cleanup Strategy
Clear pending list at:
- `storagePermissionDenied` (user refused)
- Any successful completion (when all files processed or explicit completion call)

## Implementation Steps

### 1. Java Changes (GoNativeActivity.java)
- Add static boolean `permissionRequested = false`
- In `createFileInDownloads` (line 1161-1167):
  - Check permission flag before `requestPermissions`
  - Return `null` if already requested (don't re-request)
- In `onRequestPermissionsResult` (line 1886-1899):
  - After requestCode 123 check, add:
    ```java
    boolean granted = grantResults != null && grantResults.length > 0
        && grantResults[0] == PackageManager.PERMISSION_GRANTED;
    lifecycleEvent(granted ? "storagePermissionGranted" : "storagePermissionDenied");
    ```

### 2. Go Changes (for_android.go)
- Add constant: `var ErrPermissionPending = errors.New("permission request pending")`
- Add `pendingSaves sync.Map` (global variable)
- Add `PendingSave` struct (src, dest, lu, fe, w)
- Modify `CreateFileInDownloads` (line 1000-1009):
  - Return `("", ErrPermissionPending)` when result is empty
- Add `AddPendingSave(src, dest string, lu fyne.ListableURI, fe *fyne.Container, w fyne.Window)`:
  - Store in `pendingSaves` with src as key
- Add `processPendingSaves()`: iterate sync.Map, call save functions
- Add `clearPendingSaves()`: Range over sync.Map and delete all entries

### 3. Go Changes (recv.go)
- In mass save loop (forEachFileEntry around line 1211):
  - When `ChildDownload` returns error
  - Check `errors.Is(err, ErrPermissionPending)`
  - If true: call `AddPendingSave` with (src, child, lu, fe, w)
  - Skip further processing for this file

### 4. Go Changes (log.go)
- In export log handler (line 236-239):
  - Same pattern as recv.go

### 5. Go Changes (send.go)
- Add lifecycle cases (after line 1640):
  ```go
  case "storagePermissionGranted":
      processPendingSaves()
  case "storagePermissionDenied":
      clearPendingSaves()
      // Note: needs access to window w for toast
  ```

### 6. Go Changes (for_android0.go)
- Add stubs for non-Android builds:
  ```go
  func AddPendingSave(_, _ string, _ fyne.ListableURI, _ *fyne.Container, _ fyne.Window) {}
  var ErrPermissionPending = errors.New("permission request pending")
  // processPendingSaves and clearPendingSaves can be internal (not exported)
  ```

## Validation

1. **Pre-API 29 device:** Tap mass save → permission dialog appears → grant → all files save correctly
2. **Pre-API 29 device:** Tap mass save → permission dialog → deny → error toast, no files save
3. **Post-API 29 device:** Mass save works without permission request (existing behavior)
4. **Non-Android platform:** Code compiles and works normally (stubs prevent build errors)
5. **Multiple files in cart:** All files processed or all fail together (no partial save)

## Notes

- Uses existing `lifecycleFromJava` channel infrastructure (send.go:1614-1641)
- Requires window `w` context in `PendingSave` for UI updates/toasts
- `sync.Map` handles concurrent access without mutex
- Error handling via `errors.Is()` is idiomatically correct in Go