# План: починить ft.Refresh() вместо пересоздания дерева

## Проблема

`ft.Refresh()` не обновляет дерево при WS refresh по двум причинам:

1. **`singleflight` дедупликация** — `requestGroup.DoChan("refresh", ...)` на строке 673. Если PROPFIND из `createWebDAVTree` ещё выполняется, новый вызов получит тот же (устаревший) результат.
2. **`isRefreshing` флаг** — строки 653-656. Если предыдущий refresh в процессе, новый молча отбрасывается.

## Решение

Добавить метод `ForceRefresh()` в `webdavclient.go` который:
- Сбрасывает `singleflight` через `requestGroup.Forget("refresh")`
- Не проверяет `isRefreshing` (принудительный refresh)
- Не проверяет debounce (WS refresh — это внешний сигнал, данные точно изменились)

### Изменения в `webdavclient.go`

Добавить метод `ForceRefresh()`:

```go
func (t *WebDAVFileTree) ForceRefresh() {
    if at != nil {
        if atSI := at.SelectedIndex(); atSI != SENDi {
            return
        }
    }

    t.requestGroup.Forget("refresh")
    t.lastRefresh = time.Time{} // сброс debounce

    go func() {
        ch := t.requestGroup.DoChan("refresh", func() (interface{}, error) {
            // ... та же логика что в Refresh() — вынести в общую функцию
        })

        select {
        case result := <-ch:
            if result.Err != nil {
                log.Errorf("[ForceRefresh] Failed: %v", result.Err)
                return
            }
            caches := result.Val.(struct {
                listCache map[widget.TreeNodeID][]widget.TreeNodeID
                nodeCache map[widget.TreeNodeID]*WebDAVFileNode
            })
            fyne.Do(func() {
                t.mu.Lock()
                t.listCache = caches.listCache
                t.nodeCache = caches.nodeCache
                t.mu.Unlock()
                t.Tree.Refresh()
                t.OpenAllBranches()
            })
        case <-appCtx.Done():
            return
        }
    }()
}
```

Но чтобы не дублировать код, **вынести тело refresh в общий метод**:

```go
func (t *WebDAVFileTree) startRefresh() {
    go func() {
        ch := t.requestGroup.DoChan("refresh", func() (interface{}, error) {
            // ... PROPFIND + loadChildren + build caches
        })
        select {
        case result := <-ch:
            // ... update caches + Tree.Refresh() + OpenAllBranches()
        case <-appCtx.Done():
            return
        }
    }()
}
```

Тогда:
- `Refresh()` — guard (isRefreshing, debounce) → `startRefresh()`
- `ForceRefresh()` — `Forget("refresh")` + reset debounce → `startRefresh()`

### Изменения в `send.go`

1. В `onRefresh` callback вернуть `ft.ForceRefresh()` вместо `refreshWebDAVTree`:
```go
func() {
    fyne.Do(func() {
        if ft, ok := scroller.Content.(*WebDAVFileTree); ok {
            ft.ForceRefresh()
        }
    })
},
```

2. Убрать `refreshWebDAVTree` func — больше не нужна
3. Вернуть `scroller.Content = createWebDAVTree(proxyURL)` + `de.Bounce(...)` в `switchToWebDAVTree`

## Файлы

| Файл | Изменение |
|------|-----------|
| `webdavclient.go` | Добавить `startRefresh()` (общая логика), `ForceRefresh()` (сброс singleflight + debounce) |
| `send.go` | `onRefresh` → `ft.ForceRefresh()`, убрать `refreshWebDAVTree`, вернуть прямое создание дерева |
