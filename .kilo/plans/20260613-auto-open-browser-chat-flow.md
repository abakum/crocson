# Автооткрытие браузера по чату: фикс сценария «получатель шлёт первым»

## Топология

Оба пира крутят вкладку SEND (`send.go`).

- **Отправитель (хост)** — держит WebDAV-сервер **локально** (`davServer.Start`),
  подключён к нему **без туннеля**. `davServer.IsActive()==true`, `IsRemote()==false`,
  `IsTCPForwardingActive()==false`.
- **Получатель (remote-клиент)** — подключён к **тому же** серверу **через туннель**.
  `davServer.IsRemote()==true`, `IsActive()==false`.

Чат живёт на WebDAV-сервере. `broadcastChatMessage` (`http.go:73`) рассылает сообщение
**всем** WS-клиентам `/api/chat/ws` (браузеры обоих пиров + `wsRefreshRemote` каждого пира),
не различая отправителя.

Автооткрытие (`send.go:728-735`): `wsRefreshRemote.onChatMessage` → при первом сообщении
`OpenURL(chatURL)` (флаг `opened`).

---

## Наблюдаемое поведение

- **Отправитель шлёт первым → автооткрытие на получателе.** РАБОТАЕТ. ✅
  У получателя guard `send.go:77-80` ложен (`IsRemote()==true`) → `wsRefreshRemote` живёт →
  получает сообщение → `onChatMessage` → `OpenURL`.
- **Получатель шлёт первым → автооткрытие на отправителе.** НЕ РАБОТАЕТ. ❌

### Почему не работает (корень)

`switchToWebDAVTree` (`send.go:772` → таймер `updateLink`, ветка `davServer.Start`,
т.к. `davServer.IsRemote()==false`) запускает `wsRefreshRemote`. Но в первом же проходе
цикла срабатывает guard:

```go
// send.go:77-80
if !davServer.IsRemote() && !davServer.IsTCPForwardingActive() {
    log.Debugf("[ws-refresh] tunnel down, stopping")
    return
}
```

У хоста `IsRemote()==false` и `IsTCPForwardingActive()==false` → условие истинно → `return`
**до `Dial`**. WS-клиент хоста никогда не подключается к `/api/chat/ws`, поэтому сообщения
от получателя до `onChatMessage` хоста не доходят. `chatURL` при этом задан корректно
(`send.go:706-709`).

Guard задуман как «остановить `wsRefreshRemote`, когда туннель упал» (cleanup). Но он
опирается на состояние **локального** `davServer`, а для хоста (без туннеля) это условие
истинно всегда → клиент убивается сразу после старта.

---

## Ограничение (не сломать!): оповещение об обновлении файлов

Оповещение о файлах сейчас работает правильно и его трогать **нельзя**:

- Хост ловит локальную файловую операцию **серверным хуком** `webdavHandler.Logger`
  (`webdav.go:246-256`) → `onFileTreeRefresh` → обновляет дерево **локально** +
  `broadcastRefresh()` remote-клиентам.
- Эхо-реакцию хоста на собственную рассылку `{"cmd":"refresh"}` подавляет `onRefresh = nil`
  при `IsActive()` (`send.go:717-726`). В read-loop `send.go:120-126` refresh → `onRefresh`
  (`nil` у хоста) → no-op.
- Remote-клиент: `onRefresh` задан → `ForceRefresh`.

**Фикс ниже НЕ должен менять это поведение.** В частности, `onRefresh` для хоста остаётся
`nil`, и `{"cmd":"refresh"}` по-прежнему игнорируется хостом.

---

## Фикс (минимальный, guard)

### `send.go:77-80` — добавить `!davServer.IsActive()` в условие

```go
// было
if !davServer.IsRemote() && !davServer.IsTCPForwardingActive() {
    log.Debugf("[ws-refresh] tunnel down, stopping")
    return
}

// стало
if !davServer.IsActive() && !davServer.IsRemote() && !davServer.IsTCPForwardingActive() {
    log.Debugf("[ws-refresh] tunnel down, stopping")
    return
}
```

### Почему это безопасно (включая оповещение о файлах)

- **Хост:** `IsActive()==true` → guard ложный → `wsRefreshRemote` подключается к своему
  `proxyURL` (локальный http URL + `/api/chat/ws`) и живёт, пока активен сервер.
  - refresh: `onRefresh` остаётся `nil` (`send.go:718`) → эхо refresh по-прежнему
    подавлено. **Оповещение о файлах не меняется.** ✅
  - chat: `onChatMessage` срабатывает на приходящее сообщение → `OpenURL(chatURL)`.
    Сценарий B починен. ✅
- **Remote-клиент:** ничего не меняется — `IsRemote()==true` и так пропускал guard.
- **Остановка:** `WebDAVServer.Stop()` → `stopLocked()` (`webdav.go:558`) → `active=false`
  → guard становится истинным → `wsRefreshRemote` корректно завершается. Плюс отмена
  `wsCtx` (`send.go:714-715`, потомок `appCtx`). Нет утечки горутин.

### Без изменений
- `broadcastChatMessage` (`http.go:73`), `handleChatWS` (`http.go:136`).
- Логика автооткрытия и флаг `opened` (`send.go:728-735`).
- `onRefresh` / `scRefresh` / `broadcastRefresh` / `onFileTreeRefresh`.
- `recv.go`, `directory.html`.

---

## Проверка

1. `go build ./...` / `go vet ./...`.
2. Два инстанса SEND: один хостит локально (отправитель), второй — клиент через туннель (получатель).
3. **B (был сломан):** получатель первым пишет в чат → на отправителе открывается браузер
   на `chatURL`. В логе `[ws-refresh] connected ...` (а НЕ `tunnel down, stopping`).
4. **A (регресс):** отправитель первым пишет в чат → на получателе открывается браузер.
5. **Оповещение о файлах (контроль регресса):** remote-клиент заливает/удаляет файл на
   WebDAV → дерево хоста обновляется **локально** (без двойного обновления), дерево
   получателя обновляется через refresh; обратной связи/цикла нет.
6. Остановка/переключение SEND-вкладки хоста → `wsRefreshRemote` завершается, рост
   `NumGoroutine` в логе отсутствует.

## Известный побочный эффект (не блокирующий, вне рамок задачи)
Хост теперь получает и свои собственные чат-сообщения (из своего браузера) → `opened`
может «съесться» сообщением хоста, и единственное автооткрытие сработает не от remote.
Если потребуется «подавление реакции на локальное сообщение чата» — это отдельная задача
(нужен признак источника: маркер `self` в URL/сообщении, метка туннельного WS-соединения
от `tcp_forwarder`, или семантика флага `opened` при собственном `OpenURL`).
