# Анализ реализации WebDAV и вебчата через croc туннель

## Общая архитектура

Проект crocson — GUI-клиент (Fyne framework) для утилиты передачи файлов `croc`. Встроенный WebDAV-сервер обеспечивает доступ к передаваемым файлам через браузер, а TCP-форвардинг через croc-туннель позволяет получателю файла просматривать каталог отправителя через интернет без прямого IP-доступа.

## 1. WebDAV-сервер (`webdav.go`)

### WebDAVServer
- Глобальный инстанс `davServer` создаётся в `main.go:186` через `NewWebDAVServer()`
- **Запуск**: `davServer.Start(addr, root, useTLS, addrs...)` — запускает HTTP-сервер с `handlerRouter` в качестве обработчика
- **Корневой каталог**: `join()` = `{tempDir}/send` — туда отправитель складывает файлы
- **TLS**: Опционально генерируется детерминированный самоподписанный сертификат (захардкоженный RSA-ключ + SHA-256 от IP-адресов). Один и тот же IP → один и тот же сертификат → пользователь подтверждает исключение один раз

### handlerRouter (`webdav.go:268`)
Единая точка входа для всех HTTP-запросов:
- `/api/chat/ws` → `handleChatWS` — WebSocket чат
- `/api/messages` GET/POST — REST API чата
- `/api/call/*` → `handleCallAPI` — видеозвонки
- `/videocall.html` → `serveVideoCallHTML`
- Всё остальное → `localHandler` (WebDAV handler с directory listing)

### WebDAVWithDirectoryListing (`webdav.go:67`)
Обёртка над стандартным `webdav.Handler`:
- GET-запрос на директорию → HTML-страница со списком файлов (directory listing)
- GET-запрос на файл → правильный Content-Type (через go-mime), поддержка streaming для audio/video
- Защита от path traversal
- Favicon (/favicon.ico) из встроенной иконки приложения

### ResolvingFileSystem (`webdavlink.go`)
Кастомная `webdav.FileSystem` с поддержкой симлинков:
- `ResolvingFileSystem` используется если корневой каталог = `send`
- Разрешает симлинки (псевдоссылки) в пути — позволяет показывать файлы из других каталогов
- Виртуальные пути (через симлинки) — только для чтения (маскируются write-биты)

## 2. TCP-форвардинг через croc туннель (`tcp_forwarder.go` + `webdav.go:599`)

### Как это работает

**Задача**: Получатель файла хочет увидеть WebDAV-каталог отправителя через интернет, но у отправителя может не быть белого IP.

**Решение**: Проброс TCP-соединений через уже существующий croc relay.

### Установка туннеля — `EnableTCPForwarding(client)` (`webdav.go:600`)

1. Берёт адрес relay из `client.Options.RelayAddress` (например `croc.schollz.com:9009`)
2. Создаёт **2 TCP-соединения** к relay на портах `basePort+2` и `basePort+3` с room-именами `{roomName}-1` и `{roomName}-2` (через `tcp.ConnectToTCPServer`)
3. Создаёт `TCPForwarder(conns, isSender, key, localServerAddr)` — key = shared encryption key от croc

**На стороне отправителя (IsSender=true)**:
- TCPForwarder читает из туннеля → при `ForwardMsgOpen` подключается к локальному WebDAV-серверу (`localServerAddr`) → двунаправленная пересылка данных
- Соединения мультиплексируются: каждый TCP-коннект получает уникальный `connID`

**На стороне получателя (IsSender=false)**:
- Останавливает свой WebDAV-сервер
- Создаёт локальный TCP-listener на том же порту
- `acceptLocalConnections` принимает входящие соединения → через `ForwardConnection` отправляет в туннель → данные приходят к отправителю → он подключается к своему WebDAV → ответ идёт обратно

### Протокол TCPForwarder (`tcp_forwarder.go`)

**Формат пакета** (16 байт заголовок + payload):
```
[0:8]   connID   uint64 (little-endian)
[8]     msgType  byte
[9:12]  (padding)
[12:16] payloadLen uint32 (little-endian)
[16:]   payload  []byte
```

**Типы сообщений**:
- `ForwardMsgOpen (0x01)` — открыть новое TCP-соединение
- `ForwardMsgData (0x02)` — данные соединения
- `ForwardMsgClose (0x03)` — закрыть соединение

**Шифрование**: Каждый пакет шифруется через `crypt.Encrypt(packet, key)` / `crypt.Decrypt` с shared key от croc

**Мультиплексирование**: Два канала `conns[0]` и `conns[1]`:
- `conns[0]` — для управляющих сообщений (Open, Close)
- `conns[1]` — для данных (Data)

**Буферизация**: Данные могут прийти до `ForwardMsgOpen` — они буферизуются в `pendingData` с таймаутом 30 сек

### Триггер активации

В `send.go:1097-1108` и `recv.go:763-774`:
```go
if client.Step1ChannelSecured {
    if davServer.IsActive() {
        if !davServer.IsTCPForwardingActive() {
            davServer.EnableTCPForwarding(client)
        }
    }
}
```
Туннель активируется **автоматически** после установления защищённого канала croc (Step1), если WebDAV-сервер активен.

## 3. Веб-чат (`http.go`)

### REST API
- `GET /api/messages` — получить все сообщения (опционально `?since=N` для пагинации)
- `POST /api/messages` — отправить сообщение (JSON: `{text, sender}`)

### WebSocket чат
- `WS /api/chat/ws` — upgrade до WebSocket
- Поддержка ping/pong (каждые 15 сек)
- При подключении клиент получает историю сообщений
- Входящие сообщения рассылаются всем подключённым клиентам (broadcast)
- Graceful shutdown через `appCtx`

### ChatStorage (`http.go:213`)
- In-memory хранение, `sync.RWMutex` для потокобезопасности
- Каждое сообщение: ID (unix nano), Text, Sender, Timestamp

### Интеграция с WebDAV
Чат встроен в HTML directory listing (`directory.html`):
- Секция чата обёрнута в маркеры `<!--PH:CHAT_START-->` / `<!--PH:CHAT_END-->`
- Для локальных IP — чат виден, для удалённых — секция удаляется
- Polling в `switchToWebDAVTree` (`send.go:624`): опрашивает `/api/messages?since=N` каждые 2 сек и auto-открывает браузер при первом сообщении

## 4. Видеозвонок (`videocall.go`)

Отдельный функционал, интегрированный в тот же HTTP-сервер:
- REST API: `/api/call/create`, `/api/call/join`, `/api/call/wait`, `/api/call/room`
- WebSocket: `/api/call/ws?room=X&peer=Y` — доставка медиа-чанков (WebM)
- Ring buffer чанков (до 300 на пира), бинарный поиск по индексу
- Серверная запись видео в файлы WebDAV-root
- HTML-страница `/videocall.html`

## 5. Поток данных (сценарий)

```
Отправитель                        Relay                     Получатель
    │                                │                           │
    │  croc file transfer            │                           │
    │───────────────────────────────>│──────────────────────────>│
    │  Step1: channel secured        │                           │
    │                                │                           │
    │  davServer.EnableTCPForwarding │                           │
    │  conns[0] → relay:9011 room-1  │                           │
    │  conns[1] → relay:9012 room-2  │  davServer.EnableTCPForwarding
    │                                │<── conns[0] room-1 ──────│
    │                                │<── conns[1] room-2 ──────│
    │                                │                           │
    │  Браузер получателя открывает http://localhost:PORT/       │
    │                                │                           │
    │  HTTP GET /                    │  TCPForwarder             │
    │<─────── ForwardMsgOpen ────────│<── ForwardMsgOpen ────────│
    │  dial localhost WebDAV         │                           │
    │  HTTP response (HTML)          │                           │
    │──────── ForwardMsgData ──────>│─── ForwardMsgData ────────>│
    │                                │                           │
    │  Браузер показывает файлы, чат, видеозвонки                │
```

## Ключевые файлы

| Файл | Назначение |
|------|-----------|
| `webdav.go` | WebDAV-сервер, TLS, TCP-форвардинг (менеджмент) |
| `webdavclient.go` | WebDAV-клиент, дерево файлов в GUI |
| `webdavlink.go` | ResolvingFileSystem (симлинки, виртуальные пути) |
| `tcp_forwarder.go` | Мультиплексированный TCP-форвардер с шифрованием |
| `http.go` | Чат (REST+WS), directory listing HTML |
| `videocall.go` | Видеозвонки (WebRTC-подобный signalling через WS) |
| `send.go` | Вкладка отправки, запуск WebDAV, активация туннеля |
| `recv.go` | Вкладка приёма, активация туннеля на стороне получателя |
| `directory.html` | HTML-шаблон для directory listing + чат |
| `videocall.html` | HTML-страница видеозвонка |
