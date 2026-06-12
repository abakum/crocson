# Auto-refresh remote WebDAV tree when files change

## Problem

When files are added/deleted on the WebDAV server:
- The web page (HTML directory listing) refreshes via WebSocket `broadcastRefresh()` — works
- The Fyne `WebDAVFileTree` on the remote peer does NOT refresh — broken

The sender's `onFileTreeRefresh()` only refreshes the local tree + sends WebSocket to browsers. The receiver's Fyne tree is a client widget with no push notification channel.

## Solution

Add periodic auto-refresh to `WebDAVFileTree` when the tree is showing remote files (i.e. when `davServer.IsRemote()` is true). A lightweight PROPFIND every 5 seconds to detect changes.

## Implementation — `webdavclient.go` only

### Add auto-refresh ticker to `WebDAVFileTree`

Add fields:
```go
type WebDAVFileTree struct {
    ...
    autoRefreshTicker *time.Ticker
    autoRefreshDone   chan struct{}
    isRemote          bool
}
```

### In `NewWebDAVFileTree`, start auto-refresh goroutine if remote

After creating the tree, if `davServer.IsRemote()`:
```go
if davServer.IsRemote() {
    tree.isRemote = true
    tree.autoRefreshTicker = time.NewTicker(5 * time.Second)
    tree.autoRefreshDone = make(chan struct{})
    go func() {
        for {
            select {
            case <-tree.autoRefreshDone:
                return
            case <-tree.autoRefreshTicker.C:
                fyne.Do(func() {
                    tree.Tree.Refresh()
                })
            }
        }
    }()
}
```

### Add `StopAutoRefresh()` method

```go
func (t *WebDAVFileTree) StopAutoRefresh() {
    if t.autoRefreshTicker != nil {
        t.autoRefreshTicker.Stop()
        t.autoRefreshTicker = nil
    }
    if t.autoRefreshDone != nil {
        close(t.autoRefreshDone)
        t.autoRefreshDone = nil
    }
}
```

### Stop old tree when switching

In `switchToWebDAVTree` (send.go), before replacing `scroller.Content`:
```go
if oldTree, ok := scroller.Content.(*WebDAVFileTree); ok {
    oldTree.StopAutoRefresh()
}
scroller.Content = createWebDAVTree(proxyURL)
```

## Alternative considered

Using the existing chat long-poll to detect changes — rejected because it's tightly coupled to message count, not file changes. Periodic PROPFIND is simpler and more reliable.
