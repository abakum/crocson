# Fix WS listener: history skip, goroutine leak, repeated browser open

## Root Causes

1. **History triggers `onMessage`** — on WS connect, server sends `[]Message` array. Client tries `json.Unmarshal` into `{Cmd string}`, fails (array ≠ object), falls through to `onMessage()` → opens browser on connect.
2. **Multiple WS goroutines** — each `switchToWebDAVTree()` starts a new `wsListenRemote` without stopping the previous one. Old goroutines leak forever.
3. **`chatOpened` reset** — `switchToWebDAVTree` always does `chatOpened.Store(false)`. Combined with (1), browser opens on every tree rebuild (from `scRefresh` → `switchToWebDAVTree`).
4. **No logging on success** — can't tell if WS connection was established.

## Fixes

### 1. `wsListenRemote` — skip history (JSON arrays)

In the read loop, skip messages that are JSON arrays (history `[]Message`):

```go
if len(data) > 0 && data[0] == '[' {
    continue // history array, skip
}
```

This goes before the `json.Unmarshal` check.

### 2. Cancel previous WS listener

Add a `cancelWS` variable (closure) next to existing vars in `sendTabItem`:

```go
cancelWS = func() {}
```

In `wsListenRemote`, accept a done channel instead of checking `appCtx` directly. Use a `context.WithCancel`:

Change `wsListenRemote` signature to accept a context:

```go
func wsListenRemote(ctx context.Context, httpURL string, onRefresh func(), onMessage func()) {
```

In `switchToWebDAVTree`, before calling `wsListenRemote`:

```go
if cancelWS != nil {
    cancelWS()
}
var wsCtx context.Context
wsCtx, cancelWS = context.WithCancel(appCtx)
go wsListenRemote(wsCtx, proxyURL.String(), ...)
```

In `wsListenRemote`, replace all `appCtx.Done()` with `ctx.Done()` and add `ctx.Err()` check in the loop.

### 3. Don't reset `chatOpened` on every `switchToWebDAVTree`

Move `chatOpened.Store(false)` — only reset it when connection type changes (new tunnel), not on every tree rebuild. Actually simplest: keep the reset, but since history is now skipped (fix 1), it won't matter — the browser will only open on real new messages.

### 4. Add logging

```go
log.Debugf("[ws-listen] connected to %s", wsURL)
```

After successful dial.

## Files to modify

| File | Change |
|------|--------|
| `send.go` | Add `cancelWS` var, update `wsListenRemote` signature to accept `ctx`, skip history arrays, add logging, cancel prev listener in `switchToWebDAVTree` |

No changes to `http.go` or `webdav.go`.

## Updated `wsListenRemote`

```go
func wsListenRemote(ctx context.Context, httpURL string, onRefresh func(), onMessage func()) {
    wsScheme := "ws"
    if strings.HasPrefix(httpURL, "https") {
        wsScheme = "wss"
    }
    wsURL := strings.Replace(httpURL, "http", wsScheme, 1) + "/api/chat/ws"

    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        if !davServer.IsRemote() && !davServer.IsTCPForwardingActive() {
            log.Debugf("[ws-listen] tunnel down, stopping")
            return
        }

        dialer := websocket.Dialer{
            TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
        }
        conn, _, err := dialer.Dial(wsURL, nil)
        if err != nil {
            log.Debugf("[ws-listen] dial error: %v", err)
            select {
            case <-ctx.Done():
                return
            case <-time.After(3 * time.Second):
                continue
            }
        }
        log.Debugf("[ws-listen] connected to %s", wsURL)

        for {
            select {
            case <-ctx.Done():
                conn.Close()
                return
            default:
            }

            _, data, err := conn.ReadMessage()
            if err != nil {
                conn.Close()
                break
            }

            if len(data) > 0 && data[0] == '[' {
                continue // history array, skip
            }

            var cmd struct{ Cmd string `json:"cmd"` }
            if json.Unmarshal(data, &cmd) == nil && cmd.Cmd == "refresh" {
                if onRefresh != nil {
                    onRefresh()
                }
                continue
            }

            if onMessage != nil {
                onMessage()
            }
        }
        conn.Close()
    }
}
```

## Updated `switchToWebDAVTree` (relevant section)

```go
if cancelWS != nil {
    cancelWS()
}
var wsCtx context.Context
wsCtx, cancelWS = context.WithCancel(appCtx)
go wsListenRemote(wsCtx, proxyURL.String(),
    func() { ... onRefresh ... },
    func() { ... onMessage ... },
)
```

And add `cancelWS func()` to the var block at top of `sendTabItem`.
