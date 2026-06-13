# Fix: удалить избыточный `browserOpened` gate (восстановить ретраи OpenURL)

## Симптом
На андроиде (отправитель, тап по корню WebDAV-дерева) лог показывает:
```
[createWebDAVTree] OnSelected: http://127.0.0.1:8080
[createWebDAVTree] Opening URL: http://127.0.0.1:8080
```
дважды, но браузер не открывается. На десктопе открывается.

## Почему gate теперь избыточен
`openBrowser` / `browserOpened` существовали, чтобы открыть браузер **один раз**
за сессию чата (два источника: тап по узлу дерева `OnSelected` и входящее
сообщение `wsChat`). Двойное открытие давало бы два таба.

Но коммит `2c2bd76` («replace broadcastClose with client-side tab deduplication»)
перенёс дедупликацию **на сторону браузера** в `directory.html`: каждый новый таб
инкрементирует localStorage-счётчик `dirTabs`; старый таб по `storage`-событию
видит больший счётчик и `window.close()`. Итог — остаётся ровно один таб
независимо от числа вызовов `OpenURL`.

Следовательно `browserOpened` больше не нужен: дубликатов табов не будет в любом
случае, а сам gate сейчас **вредит** — он ставится в `true` до вызова `OpenURL`
и ошибка выбрасывается, поэтому на андроиде (где `OpenURL` фейлится молча) флаг
навсегда остаётся `true` и блокирует все ретраи. До фикса `OnSelected` звал
`OpenURL` напрямую на каждый тап — флейковые/временные провалы можно было
«протапать» заново.

## Цель
Удалить gate полностью, вернуть прямые вызовы `OpenURL` (с логом ошибки) —
ретраи снова работают, дубликат табов отсекается клиентской дедупликацией.

## Изменения (только `send.go`)

### 1. Удалить gate из var-блока (send.go:219–237)
Убрать:
```go
cancelChatWS context.CancelFunc

browserOpenedMu sync.Mutex
browserOpened   bool
)

// openBrowser открывает URL один раз за сессию чата; true, если открыл.
openBrowser := func(url string) bool {
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

var (
```
оставить только:
```go
cancelChatWS context.CancelFunc
)
```
(т.е. закрыть первый var-блок сразу после `cancelChatWS`, убрать `openBrowser`
целиком, и вернуть `boxholder`-var-блок в исходный вид).

### 2. `createWebDAVTree.OnSelected` — прямой OpenURL (send.go:731–734)
```go
log.Debugf("[createWebDAVTree] Opening URL: %s", fullURLStr)
time.AfterFunc(100*time.Millisecond, func() {
    openBrowser(fullURLStr)
})
```
→
```go
log.Debugf("[createWebDAVTree] Opening URL: %s", fullURLStr)
time.AfterFunc(100*time.Millisecond, func() {
    if err := OpenURL(fullURLStr); err != nil {
        log.Errorf("[createWebDAVTree] OpenURL %s: %v", fullURLStr, err)
    }
})
```

### 3. `wsChat` callback в `startChatWS` — прямой OpenURL (send.go:810–814)
```go
go wsChat(ctx, httpURL, func() {
    if chatURL != "" && openBrowser(chatURL) {
        log.Debugf("[ws-chat] auto-opening browser: %s", chatURL)
    }
})
```
→
```go
go wsChat(ctx, httpURL, func() {
    if chatURL != "" {
        if err := OpenURL(chatURL); err != nil {
            log.Errorf("[ws-chat] OpenURL %s: %v", chatURL, err)
        } else {
            log.Debugf("[ws-chat] auto-opening browser: %s", chatURL)
        }
    }
})
```

### 4. Убрать сброс флага в `startChatWS` (send.go:803–805)
```go
if cancelChatWS != nil {
    cancelChatWS()
}
browserOpenedMu.Lock()
browserOpened = false
browserOpenedMu.Unlock()
```
→
```go
if cancelChatWS != nil {
    cancelChatWS()
}
```

## Итог
- Десктоп: поведение не меняется (OpenURL ок; дубликат табов режется в
  `directory.html`).
- Андроид: провал `OpenURL` теперь виден в логе как
  `[createWebDAVTree] OpenURL http://127.0.0.1:8080: <error>` / `[ws-chat] ...`,
  а повторный тап (или следующее сообщение чата) снова пытается открыть браузер.

## Проверка
- `go build ./... && go vet ./...`.
- Десктоп: один тап → 1 таб; второй тап → новый таб, старый сам закрывается
  (дедупликация `dirTabs`).
- Андроид: собрать, тапнуть корень дерева, собрать лог ошибки `OpenURL` и
  сообщить текст — по нему решим отдельный фикс intent-механизма (raw `http://`
  vs построенный `intent://` с `BROWSER`-флагами, как в `handleIOTapped`,
  applinks.go:1046).
