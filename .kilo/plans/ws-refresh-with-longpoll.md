# План: вернуть long-poll + добавить WS refresh (поэтапно)

## Цель

1. **Вернуть long-poll** для автооткрытия браузера (работал на обоих пирах)
2. **Добавить WS listener** ТОЛЬКО для приёма `{"cmd":"refresh"}` → `ft.Refresh()`
3. **WS listener только на receiver** (`!davServer.IsActive()`)

## Изменения в `send.go`

### 1. Упростить `wsListenRemote` — убрать `onMessage`

Функция больше не отвечает за чат. Только refresh:

```go
func wsRefreshRemote(ctx context.Context, httpURL string, onRefresh func()) {
    // ... тот же код, но:
    // - убран onMessage параметр
    // - всё что не {"cmd":"refresh"} — игнорируется (включая чат)
    // - conn.Close() goroutine по ctx.Done()
}
```

### 2. Вернуть long-poll goroutine в `switchToWebDAVTree`

Точно такой же, как был до изменений (строки 636-692 оригинала).

### 3. `switchToWebDAVTree` — итоговая структура

```go
switchToWebDAVTree := func() {
    if link.URL == nil { return }
    if davServer.IsActive() || davServer.IsTCPForwardingActive() || davServer.IsRemote() {
        _, ccn, proxyURL, _ := isDAV(link.URL.String())
        chatURL = ccn
        chatOpened.Store(false)

        // Long-poll для чата (оригинальный код, работает на обоих пирах)
        go func() {
            errCount := 0
            resp, err := insecureHTTPClient.Get(proxyURL.String() + "/api/messages")
            // ... весь оригинальный long-poll код ...
        }()

        // WS refresh — ТОЛЬКО на receiver (не sender)
        if !davServer.IsActive() {
            if cancelWS != nil {
                cancelWS()
            }
            var wsCtx context.Context
            wsCtx, cancelWS = context.WithCancel(appCtx)
            go wsRefreshRemote(wsCtx, proxyURL.String(), func() {
                fyne.Do(func() {
                    if ft, ok := scroller.Content.(*WebDAVFileTree); ok {
                        ft.Refresh()
                    }
                })
            })
        }

        scroller.Content = createWebDAVTree(proxyURL)
        de.Bounce(ti.Content.Refresh)
    }
}
```

### 4. Убрать `chatOpened` и `chatURL` из http.go (если были перенесены)

Нет, они остались как глобальные переменные — не трогаем.

## Файлы

| Файл | Изменение |
|------|-----------|
| `send.go` | Переименовать `wsListenRemote` → `wsRefreshRemote`, убрать `onMessage`, вернуть long-poll, добавить условие `!davServer.IsActive()` для WS |

## Ключевые моменты

- Long-poll работает на **обоих пирах** (sender тоже подписан на свои же сообщения через `/api/messages/wait`)
- WS refresh работает **только на receiver** — sender обновляет дерево через `scRefresh` → `switchToWebDAVTree` напрямую
- `cancelWS` корректно закрывает WS через `conn.Close()` goroutine
- `cancelWS` var остаётся в var block
