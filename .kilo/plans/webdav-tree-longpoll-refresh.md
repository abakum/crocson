# Auto-refresh remote WebDAV tree via long-poll

## Problem

When files change on the WebDAV server (sender side), the HTML page refreshes via WebSocket but the receiver's Fyne `WebDAVFileTree` doesn't update.

## Solution

Add a `/api/files/wait` long-poll endpoint (same pattern as existing `/api/messages/wait`) and make the receiver's tree poll it. When files change, `onFileTreeRefresh` notifies the channel. The receiver's long-poll goroutine wakes up and refreshes the tree.

## Implementation

### 1. Add file change notifier — `http.go`

Add a simple counter + notify channel (same pattern as `ChatStorage`):

```go
var fileChangeStore = struct {
    mu      sync.RWMutex
    version int
    notifyCh chan struct{}
}{
    notifyCh: make(chan struct{}, 1),
}

func notifyFileChange() {
    fileChangeStore.mu.Lock()
    fileChangeStore.version++
    fileChangeStore.mu.Unlock()
    select {
    case fileChangeStore.notifyCh <- struct{}{}:
    default:
    }
}
```

### 2. Add `/api/files/wait` endpoint — `http.go`

```go
func handleWaitForFileChanges(w http.ResponseWriter, r *http.Request) {
    // same pattern as handleWaitForMessages
    sinceStr := r.URL.Query().Get("since")
    since, _ := strconv.Atoi(sinceStr)

    timeout := 30 * time.Second // longer than messages — files change less often
    ctx, cancel := context.WithTimeout(r.Context(), timeout)
    defer cancel()

    for {
        fileChangeStore.mu.RLock()
        v := fileChangeStore.version
        fileChangeStore.mu.RUnlock()

        if v > since {
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]int{"version": v})
            return
        }

        select {
        case <-fileChangeStore.notifyCh:
            continue
        case <-ctx.Done():
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]int{"version": v})
            return
        }
    }
}
```

### 3. Register route — `webdav.go` `handlerRouter`

Add after `/api/messages/wait`:

```go
if r.URL.Path == "/api/files/wait" {
    handleWaitForFileChanges(w, r)
    return
}
```

### 4. Call `notifyFileChange()` when files change — `webdav.go`

In `createLocalHandler` Logger, alongside existing `onFileTreeRefresh()`:

```go
if r.Method == "PUT" || r.Method == "DELETE" || r.Method == "MKCOL" || r.Method == "MOVE" {
    if onFileTreeRefresh != nil {
        onFileTreeRefresh()
    }
    notifyFileChange()
}
```

### 5. Start long-poll in `switchToWebDAVTree` — `send.go`

Add a goroutine alongside the existing message long-poll that polls `/api/files/wait` and refreshes the tree:

```go
// File change long-poll
go func() {
    version := 0
    for {
        select {
        case <-appCtx.Done():
            return
        default:
        }
        resp, err := insecureHTTPClient.Get(
            fmt.Sprintf("%s/api/files/wait?since=%d", proxyURL.String(), version))
        if err != nil {
            select {
            case <-appCtx.Done():
                return
            case <-time.After(5 * time.Second):
                continue
            }
        }
        var result struct{ Version int }
        json.NewDecoder(resp.Body).Decode(&result)
        resp.Body.Close()

        if result.Version > version {
            version = result.Version
            fyne.Do(func() {
                if ft, ok := scroller.Content.(*WebDAVFileTree); ok {
                    ft.Refresh()
                }
            })
        }
    }
}()
```

### 6. Stop old tree when switching — `send.go`

Before `scroller.Content = createWebDAVTree(proxyURL)` in `switchToWebDAVTree`, no special handling needed since the old goroutines exit when `appCtx` is done. But cancel via a per-tree context would be cleaner. Simplest: just let goroutines die naturally when tree is replaced (they reference old tree which gets GC'd). The `Refresh()` on old tree is a no-op since it's no longer in the scroller.

### Files to modify

| File | Change |
|------|--------|
| `http.go` | Add `fileChangeStore`, `notifyFileChange()`, `handleWaitForFileChanges()` |
| `webdav.go` | Register `/api/files/wait` route in `handlerRouter`; call `notifyFileChange()` in Logger |
| `send.go` | Add file-change long-poll goroutine in `switchToWebDAVTree` |
