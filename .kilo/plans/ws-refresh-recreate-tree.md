# План: пересоздание дерева вместо ft.Refresh()

## Проблема

`ft.Refresh()` не обновляет виджет визуально — дерево нужно пересоздавать через `createWebDAVTree()` + замена `scroller.Content`.

## Решение

Вместо `ft.Refresh()` в `onRefresh` callback вызывать `switchToWebDAVTree()` — она уже пересоздаёт дерево.

Но `switchToWebDAVTree` делает лишнее при вызове из WS:
1. Стартует новый long-poll goroutine ← накапливаются
2. Стартует новый WS listener ← `cancelWS` + новый goroutine (это ок)
3. Пересоздаёт дерево ← это то что нужно

### Вариант: вынести обновление дерева в отдельную функцию

```go
refreshWebDAVTree := func() {
    if link.URL == nil { return }
    if davServer.IsActive() || davServer.IsTCPForwardingActive() || davServer.IsRemote() {
        _, _, proxyURL, _ := isDAV(link.URL.String())
        scroller.Content = createWebDAVTree(proxyURL)
        de.Bounce(ti.Content.Refresh)
    }
}
```

Тогда:
- `switchToWebDAVTree` = long-poll + WS start + `refreshWebDAVTree()`
- `onRefresh` callback = `fyne.Do(refreshWebDAVTree)`
- Без накопления goroutine'ов

### Изменения в `send.go`

1. Добавить `refreshWebDAVTree` func перед `switchToWebDAVTree`
2. `switchToWebDAVTree` вызывает `refreshWebDAVTree()` вместо прямого `createWebDAVTree`
3. `onRefresh` в `wsRefreshRemote` вызывает `fyne.Do(refreshWebDAVTree)` вместо `ft.Refresh()`

### Итоговый код switchToWebDAVTree

```go
refreshWebDAVTree := func() {
    if link.URL == nil { return }
    if davServer.IsActive() || davServer.IsTCPForwardingActive() || davServer.IsRemote() {
        _, _, proxyURL, _ := isDAV(link.URL.String())
        scroller.Content = createWebDAVTree(proxyURL)
        de.Bounce(ti.Content.Refresh)
    }
}

switchToWebDAVTree := func() {
    if link.URL == nil { return }
    if davServer.IsActive() || davServer.IsTCPForwardingActive() || davServer.IsRemote() {
        _, ccn, proxyURL, _ := isDAV(link.URL.String())
        chatURL = ccn
        chatOpened.Store(false)
        
        // Long-poll для чата (работает на обоих пирах)
        go func() { /* ... оригинальный long-poll ... */ }()

        // WS refresh — только на receiver
        if !davServer.IsActive() {
            if cancelWS != nil { cancelWS() }
            var wsCtx context.Context
            wsCtx, cancelWS = context.WithCancel(appCtx)
            go wsRefreshRemote(wsCtx, proxyURL.String(),
                func() {
                    fyne.Do(refreshWebDAVTree)
                },
            )
        }

        refreshWebDAVTree()
    }
}
```

## Файлы

| Файл | Изменение |
|------|-----------|
| `send.go` | Добавить `refreshWebDAVTree`, использовать в `switchToWebDAVTree` и в `onRefresh` callback |
