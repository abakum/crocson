# Подавление автооткрытия браузера при локальном открытии (дедуп открытий)

## Симптом (из лога)

После фикса guard'а автооткрытие работает, но появляется **дублирующее открытие**:

- `22:38:12` `[createWebDAVTree] OnSelected ... Opening URL` — пользователь тапнул корень дерева
  → браузер открыт (локальное действие).
- `22:38:21` `[ws] auto-opening browser` — пришло чат-сообщение → автооткрытие сработало
  **поверх уже открытого** браузера.

## Причина

Флаг `opened`, гейтящий автооткрытие, живёт **только** внутри замыкания `wsRefreshRemote`
(`send.go:728`). Открытие браузера через тап по дереву (`createWebDAVTree.OnSelected`,
`send.go:641-643`) этот флаг **не выставляет** → последующее сообщение по WS снова триггерит
`OpenURL`. Это и есть «реакция на локальное обновление», которую нужно подавить (по образцу
того, как `onRefresh=nil` подавляет эхо refresh у хоста).

## Фикс — единый гейт «браузер уже открывали»

### 1. Общий флаг + guarded-opener в scope `sendTabItem`

Рядом с `cancelWS` (`send.go:147`) объявить:

```go
var (
    browserOpenedMu sync.Mutex
    browserOpened   bool
)

// openBrowser открывает URL ровно один раз за WS-сессию; возвращает true, если открыл.
openBrowser := func(url string) (opened bool) {
    browserOpenedMu.Lock()
    if browserOpened {
        browserOpenedMu.Unlock()
        return false
    }
    browserOpened = true
    browserOpenedMu.Unlock()
    OpenURL(url)
    return true
}
```

`sync` уже импортирован (`send.go:24`).

### 2. `createWebDAVTree` — открывать через `openBrowser`

- Сигнатуру расширить параметром-открывалкой:
  `func createWebDAVTree(webdavURL *url.URL, openFn func(string) bool) *WebDAVFileTree`.
- В `OnSelected` (`send.go:641-643`):
  ```go
  time.AfterFunc(100*time.Millisecond, func() {
      openFn(fullURLStr)
  })
  ```
- Вызов (`send.go:737`): `scroller.Content = createWebDAVTree(proxyURL, openBrowser)`.

### 3. Автооткрытие в `switchToWebDAVTree` — через `openBrowser`, без локального `opened`

`send.go:728-735`:
```go
// было
opened := false
wsRefreshRemote(wsCtx, proxyURL.String(), onRefresh, func() {
    if !opened && chatURL != "" {
        opened = true
        log.Debugf("[ws] auto-opening browser: %s", chatURL)
        OpenURL(chatURL)
    }
})

// стало
wsRefreshRemote(wsCtx, proxyURL.String(), onRefresh, func() {
    if chatURL != "" && openBrowser(chatURL) {
        log.Debugf("[ws] auto-opening browser: %s", chatURL)
    }
})
```

### 4. Сброс флага на новую WS-сессию

В начале горутины `switchToWebDAVTree` (`send.go:710`, после `cancelWS()`/создания `wsCtx`)
сбрасывать, чтобы каждое (пере)подключение дерева допускало одно открытие:
```go
browserOpenedMu.Lock()
browserOpened = false
browserOpenedMu.Unlock()
```
Это сохраняет прежнюю семантику `opened` (новый флаг на каждый запуск `wsRefreshRemote`).

## Что не трогаем

- Guard `send.go:77` (недавний фикс) — без изменений.
- `onRefresh`/`scRefresh`/`broadcastRefresh`/`onFileTreeRefresh` — оповещение о файлах не трогаем.
- `link.OnTapped` (`send.go:673-698`) — отдельный ручной путь (часто открывает файловый
  менеджер / OpenDAV, не чат-страницу); сознательно оставляем как есть. Если потребуется и
  его учитывать — пропустить корневой `OpenURL(root)` через `openBrowser` тоже.

## Поведение после фикса

| Событие | Результат |
|---|---|
| Тап по дереву (локальное открытие) | `browserOpened=true`, браузер открыт |
| Затем приходит чат-сообщение | `openBrowser` видит флаг → **дубликата нет** |
| Сообщение пришло первым (браузер не открывали) | автооткрытие сработает один раз |
| Переключение/пересоздание дерева | флаг сброшен → снова одно открытие допустимо |

## Проверка

1. `go build ./...` / `go vet ./...`.
2. Воспроизвести лог: тап по корню дерева → браузер открыт → отправить чат-сообщение →
   второго `[ws] auto-opening browser` в логе **нет**.
3. Обратный порядок: сообщение первым (без тапа) → ровно одно автооткрытие.
4. Регресс оповещения о файлах: refresh по-прежнему работает (хост не реагирует на эхо).
