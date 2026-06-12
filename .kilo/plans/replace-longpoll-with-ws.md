# Plan: Replace long-poll auto-open with WS client

## Current state

In `switchToWebDAVTree()` (`send.go:698-783`):

- **Long-poll goroutine** (lines 707-763): runs on BOTH sender and receiver. Polls `/api/messages/wait?since=N` to detect new chat messages → auto-opens browser.
- **`wsRefreshRemote`** (lines 764-779): runs ONLY on receiver (`!davServer.IsActive()`). WS client to `/api/chat/ws` handles `{"cmd":"refresh"}` for file tree refresh.

Both connect to the same remote peer through the tunnel.

## Goal

Replace the long-poll goroutine with a WS client on both sides:
- **Sender**: WS client for chat messages only (auto-open browser), no file refresh
- **Receiver**: WS client for chat messages (auto-open browser) + file refresh

## Changes — `send.go` only

### 1. Extend `wsRefreshRemote` signature (line 63)

Add `onChatMessage func()` parameter:

```go
func wsRefreshRemote(ctx context.Context, httpURL string, onRefresh func(), onChatMessage func()) {
```

In the message reading loop (after line 119), add chat message handling before the refresh check:

```go
var cmd struct{ Cmd string `json:"cmd"` }
if json.Unmarshal(data, &cmd) == nil && cmd.Cmd == "refresh" {
    log.Debugf("[ws-refresh] refresh received")
    if onRefresh != nil {
        onRefresh()
    }
    continue
}

if onChatMessage != nil {
    onChatMessage()
}
```

### 2. Replace long-poll + WS in `switchToWebDAVTree` (lines 707-779)

Remove the entire long-poll goroutine (lines 707-763) and the `if !davServer.IsActive()` block (lines 764-779). Replace with a single WS listener that always starts:

```go
go func() {
	if cancelWS != nil {
		cancelWS()
	}
	var wsCtx context.Context
	wsCtx, cancelWS = context.WithCancel(appCtx)

	var onRefresh func()
	if !davServer.IsActive() {
		onRefresh = func() {
			fyne.Do(func() {
				if ft, ok := scroller.Content.(*WebDAVFileTree); ok {
					ft.ForceRefresh()
				}
			})
		}
	}

	wsRefreshRemote(wsCtx, proxyURL.String(), onRefresh, func() {
		if chatOpened.CompareAndSwap(false, true) && chatURL != "" {
			log.Debugf("[ws] auto-opening browser: %s", chatURL)
			OpenURL(chatURL)
		}
	})
}()
```

Key points:
- `onRefresh` is `nil` on sender → `wsRefreshRemote` skips refresh callbacks
- `onChatMessage` (auto-open browser) runs on both sides
- `cancelWS` is cancelled before creating a new context, preventing stale connections

### Summary

| | Before | After |
|---|---|---|
| Sender | Long-poll HTTP | WS (chat only) |
| Receiver | Long-poll HTTP + WS (refresh) | Single WS (chat + refresh) |

**One file changed**: `send.go`
