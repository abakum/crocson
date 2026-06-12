# Оригинальный поток данных (до WS изменений)

## Актёры

| Актёр | Роль | Сервер | Порт |
|-------|------|--------|------|
| **Sender** | Хостит WebDAV, раздаёт файлы | `davServer` на `:8080` | 8080 |
| **Receiver** | Получает файлы через туннель | TCP forwarder на `:8081 → :8080` (через relay) | 8081 |

## Часть 1: Обновление дерева файлов

### Sender — локальное дерево

```
Файл изменён (PUT/DELETE/MKCOL/MOVE)
  → webdav.go:251 Logger в createLocalHandler
    → onFileTreeRefresh()
      → scRefresh() (send.go:812)
        → switchToWebDAVTree()  ← пересоздаёт дерево (PROPFIND)
        → broadcastRefresh()    ← шлёт {"cmd":"refresh"} всем WS-клиентам
```

### Sender — браузер

```
broadcastRefresh()
  → http.go:91 отправляет {"cmd":"refresh"} всем chatWSClients
    → directory.html:449 JS получает {"cmd":"refresh"}
      → refreshFileTable() ← AJAX GET текущей страницы
```

### Receiver — дерево (было через long-poll, работало ТАКЖЕ через scRefresh)

**ОРИГИНАЛЬНЫЙ поток (до моих изменений):**

```
Receiver: scRefresh()
  → switchToWebDAVTree()  ← пересоздаёт дерево (PROPFIND через туннель :8081 → :8080)
  → broadcastRefresh()    ← шлёт {"cmd":"refresh"} ВСЕМ WS-клиентам сервера
```

**Проблема:** `scRefresh()` вызывается на ОБОИХ пирах, но `onFileTreeRefresh` привязан к **sender's** WebDAV handler (webdav.go:252). Когда receiver загружает файл через туннель:

```
Receiver drag&drop → PUT через туннель :8081
  → Sender WebDAV handler получает PUT
    → Sender: onFileTreeRefresh() → scRefresh()
      → Sender: switchToWebDAVTree() + broadcastRefresh()
      → Receiver: switchToWebDAVTree() тоже вызывается!
        Потому что scRefresh = func() общий для обоих пиров
```

**Но receiver НЕ получает `onFileTreeRefresh` напрямую** — callback привязан к sender's WebDAV handler. Receiver узнаёт об изменении только если `scRefresh` вызывается на нём напрямую (например, `EnteredForeground` → `executeAll`).

**Ключевой момент:** Обновление дерева на receiver было НЕАВТОМАТИЧЕСКИМ. Receiver обновлялся только при:
- `EnteredForeground` → `executeAll` → `scRefresh()` 
- Или при следующем вызове `switchToWebDAVTree()` по какой-то другой причине

## Часть 2: Чат — автооткрытие браузера

### Отправка сообщения

```
Браузер sender'а: chatSendBtn → chatWS.send({text, sender})
  → Sender: handleChatWS → chatStore.addMessage()
    → broadcastChatMessage(msg) → все WS-клиенты получают Message JSON
```

### Получение на receiver (ОРИГИНАЛЬНЫЙ long-poll)

```
Receiver: switchToWebDAVTree()
  → go goroutine {
      GET /api/messages → initialMsgs (baseline count)
      loop {
        GET /api/messages/wait?since=lastCount  ← long-poll, ждёт до 5s
        if result.Count > lastCount:
          chatOpened.CompareAndSwap(false, true)
          OpenURL(chatURL) ← открывает http://127.0.0.1:8081 в браузере
          return
      }
    }
```

**Важно:** `chatURL = ccn = http://127.0.0.1:8081` — это корень WebDAV через туннель. Браузер открывает directory listing со встроенным чатом. Чат ВСТРОЕН в страницу directory listing (directory.html:326-331).

### Поток чата в браузере receiver'а после автооткрытия

```
Receiver browser: GET http://127.0.0.1:8081/
  → TCP forwarder → Sender: serveDirectoryListing() → HTML с чатом
    → JS: connectChatWS() → WS к ws://127.0.0.1:8081/api/chat/ws
      → TCP forwarder → Sender: handleChatWS()
        → Sender: отправляет историю сообщений (history []Message)
        → Sender: регистрирует клиента для broadcast'ов
```

## Часть 3: WS сообщение refresh в браузере

**Только в браузере** (JS в directory.html), НЕ в Go клиенте:

```
broadcastRefresh() → {"cmd":"refresh"}
  → directory.html:449 if (data.cmd === 'refresh') { refreshFileTable(); }
    → refreshFileTable() ← AJAX перезагружает список файлов
```

**Оригинальный Go клиент (receiver) НЕ использовал WS для refresh.** Он использовал long-poll `/api/messages/wait` ТОЛЬКО для чата.

## Резюме: что было и что сломалось

### Было (оригинал):

| Событие | Sender | Receiver |
|---------|--------|----------|
| Файл изменён | `onFileTreeRefresh` → `scRefresh` → пересоздаёт дерево + `broadcastRefresh` | **НЕ обновлялся автоматически** (только при `EnteredForeground`) |
| Чат сообщение | Браузер sender'а получает через WS (встроенный в directory.html) | Long-poll `/api/messages/wait` → открывает браузер → browser JS подключается к WS |
| WS refresh | Браузер обновляет файлы через `refreshFileTable()` | **Go клиент не подписан на WS** |

### Что я сделал (WS listener):

- Добавил `wsListenRemote` — Go WS клиент на receiver'е
- Подписался на `{"cmd":"refresh"}` → `ft.Refresh()` (PROPFIND)
- Подписался на chat messages → auto-open browser
- Заменил long-poll на WS

### Что сломалось:

1. **Sender тоже запускает `wsListenRemote`** (к `ws://127.0.0.1:8080`) — бессмысленно, создаёт петли
2. **`cancelWS()` не закрывает WS connection** — goroutine зависает в `ReadMessage()`, накапливаются
3. **Нет ограничения: только receiver** — sender не должен запускать WS listener
4. **History skip** работает (`data[0] == '['`), но нужно также не открывать browser при reconnect

### План исправления

1. **Запускать `wsListenRemote` только когда `!davServer.IsActive()`** (receiver side)
2. **Закрывать conn при cancel** — goroutine с `conn.Close()` по `ctx.Done()`
3. **Sender использует старую логику** — `scRefresh` → `switchToWebDAVTree` + `broadcastRefresh` (без WS listener)
