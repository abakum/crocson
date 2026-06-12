# Fix: clicking file tree root opens new tab every time

## Problem

`OnSelected` in `createWebDAVTree` (send.go:630) unconditionally calls `OpenURL()`. Each click on the root node opens a new browser tab, accumulating tabs.

## Fix — `send.go`

### 1. Send `broadcastClose()` before `OpenURL` in `OnSelected`

In `createWebDAVTree` (line 630), add `broadcastClose()` call before opening the new tab:

```go
ft.OnSelected = func(uid widget.TreeNodeID) {
    ...
    chatOpened.Store(true)
    broadcastClose()
    time.AfterFunc(100*time.Millisecond, func() {
        OpenURL(fullURLStr)
    })
    ft.Unselect(uid)
}
```

This sends `{"cmd":"close"}` to all WS-connected browser tabs, which triggers `window.close()` in JS (directory.html:445,450). The old tab closes before the new one opens.

### 2. Remove `chatOpened.Store(false)` from `switchToWebDAVTree`

Move the reset from the main body (line 711) into the WS goroutine, so tree rebuilds via `scRefresh` don't reset the flag:

```go
switchToWebDAVTree := func() {
    ...
    // Remove: chatOpened.Store(false)  ← was here
    go func() {
        if cancelWS != nil {
            cancelWS()
        }
        chatOpened.Store(false) // reset only on new WS connection
        ...
    }()
    ...
}
```

## Files changed

| File | Change |
|---|---|
| `send.go:641` | Add `broadcastClose()` before `OpenURL` in `OnSelected` |
| `send.go:711` | Move `chatOpened.Store(false)` into WS goroutine |
