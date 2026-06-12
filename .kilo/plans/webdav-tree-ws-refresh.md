# Auto-refresh remote WebDAV tree via existing WebSocket

## Approach

Reuse existing `broadcastRefresh()` which sends `{"cmd": "refresh"}` to all WS clients when files change. Add a Go WebSocket client on the receiver side that connects to `/api/chat/ws` through the tunnel and:
- Refreshes the Fyne tree when it receives `{"cmd": "refresh"}`
- Auto-opens browser on new chat messages via `broadcastChatMessage()`

This **replaces** the existing `/api/messages/wait` long-poll goroutine with a single WS connection that handles both events. The long-poll code is removed from `switchToWebDAVTree`.

## WS message types

Two types are broadcast through the same `/api/chat/ws`:
1. `broadcastRefresh()` → `{"cmd":"refresh"}` — file tree changed
2. `broadcastChatMessage(msg)` → `{"id":"...","text":"...","sender":"...","timestamp":"..."}` — new chat message

The client distinguishes by checking for the `cmd` field.

## Implementation

### 1. New function `wsListenRemote` — `send.go`

Connects to the sender's `/api/chat/ws` via tunnel, auto-reconnects on error:

```go
func wsListenRemote(wsURL string, onRefresh func(), onMessage func()) {
    // Convert http(s):// to ws(s)://
    wsScheme := "ws"
    if strings.HasPrefix(wsURL, "https") {
        wsScheme = "wss"
    }
    wsURL = strings.Replace(wsURL, "http", wsScheme, 1) + "/api/chat/ws"

    for {
        select {
        case <-appCtx.Done():
            return
        default:
        }

        // Reconnect only while tunnel is alive
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
            case <-appCtx.Done():
                return
            case <-time.After(3 * time.Second):
                continue
            }
        }

        for {
            _, data, err := conn.ReadMessage()
            if err != nil {
                if !websocket.IsUnexpectedCloseError(err, ...) {
                    log.Debugf("[ws-listen] read error: %v", err)
                }
                conn.Close()
                break // reconnect (checks tunnel in outer loop)
            }

            var cmd struct{ Cmd string `json:"cmd"` }
            if json.Unmarshal(data, &cmd) == nil && cmd.Cmd == "refresh" {
                onRefresh()
                continue
            }

            // Otherwise it's chat message(s) — trigger onMessage
            if onMessage != nil {
                onMessage()
            }
        }
        conn.Close()
    }
}
```

### 2. Replace long-poll in `switchToWebDAVTree` — `send.go`

Remove the existing message long-poll goroutine (lines 636-692) and replace with:

```go
go wsListenRemote(proxyURL.String(),
    func() {
        // onRefresh — refresh the Fyne tree
        fyne.Do(func() {
            if ft, ok := scroller.Content.(*WebDAVFileTree); ok {
                ft.Refresh()
            }
        })
    },
    func() {
        // onMessage — auto-open browser on new chat message
        if chatOpened.CompareAndSwap(false, true) && chatURL != "" {
            log.Debugf("[ws-listen] auto-opening browser: %s", chatURL)
            OpenURL(chatURL)
        }
    },
)
```

### Files to modify

| File | Change |
|------|--------|
| `send.go` | Add `wsListenRemote`, replace message long-poll in `switchToWebDAVTree` |

No changes to `http.go` or `webdav.go` — `broadcastRefresh()` already sends `{"cmd": "refresh"}` on file changes.

### Key details

- **Auto-reconnect**: Only reconnects while tunnel is alive (`davServer.IsRemote() || davServer.IsTCPForwardingActive()`). If tunnel is down, WS listener stops cleanly.
- **`gorilla/websocket`**: Already imported, used for WS server. Dialer is client-side.
- **InsecureSkipVerify**: Same as `insecureHTTPClient` — self-signed cert through tunnel.
- **`onRefresh`**: Calls `ft.Refresh()` on the current `WebDAVFileTree` which re-fetches from server via PROPFIND.
- **`onMessage`**: Replaces the separate `/api/messages/wait` long-poll — single WS connection handles both.
- **Tree nil check**: `scroller.Content.(*WebDAVFileTree)` — if tree was replaced (nil), refresh is no-op.
