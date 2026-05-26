# План: WebDAV через wormhole туннель

## Цель

Когда в настройках relay указан wormhole mailbox (`ws://`/`wss://`), mainButton (Send/Download) запускает **wormhole клиент** вместо croc клиента. Wormhole туннель — основной и единственный механизм соединения, без croc.

## Контекст

### Что уже есть

1. **crocson**: `tcp_forwarder.go` + `webdav.go:EnableTCPForwarding` — проброс WebDAV через croc relay (2 TCP-соединения, мультиплексирование, шифрование AES)
2. **wormhole-william fork** (`/home/koka/src/wormhole-william`): уже реализован tunnel API:
   - `wormhole/tunnel/protocol.go` — OPEN/DATA/CLOSE сообщения (big-endian, 13 байт заголовок)
   - `wormhole/tunnel/session.go` — `Session` с `RecordIO` интерфейсом, `readLoop`, `tunnelConn` (net.Conn)
   - `wormhole/tunnel/tunnel.go` — `Tunnel` с `Forward()`, `Serve()`, `Dial()`, `Listen()`, `Close()`, `Proxy()`
   - `wormhole/tunnel_integration.go` — `Client.CreateTunnel()` / `Client.JoinTunnel()` — PAKE + transit → tunnel.Session

### Ключевое отличие от croc туннеля

| Аспект | croc туннель | wormhole туннель |
|--------|-------------|-----------------|
| Активация | mainButton → croc PAKE → Step1 → EnableTCPForwarding | mainButton → wormhole PAKE + transit → tunnel готов |
| Основной клиент | croc (croc.Client) | wormhole (wormhole.Client) |
| Транспорт | 2 TCP к croc relay | 1 transit-соединение (direct + relay fallback) |
| Мультиплексирование | `TCPForwarder` | `tunnel.Session` (встроен в wormhole-william) |
| Шифрование | AES (TCPForwarder) | NaCl secretbox (transportCryptor) |
| Файлы | Передаются через croc | Браузер скачивает через WebDAV |
| Жизненный цикл | Пока длится передача файла | Пока пользователь не отменит |

**Wormhole туннель НЕ использует TCPForwarder** — у него свой мультиплексор и шифрование.

## Предварительные условия и жизненный цикл

### Поток wormhole режима

```
Отправитель                                    Получатель
    │                                              │
    │  1. Переключает в WebDAV режим               │
    │     (treeButton → VisibilityOffIcon)          │
    │     → WebDAV сервер стартует                  │
    │  2. Вводит secret                             │  1. Вводит тот же secret
    │  3. Нажимает mainButton "Send"                │  2. Нажимает mainButton "Download"
    │     ┌─────────────────────────────┐           │     ┌─────────────────────────────┐
    │     │  isWormholeRelay(relayAddr)  │          │     │  isWormholeRelay(relayAddr)  │
    │     │  → true → wormhole путь     │           │     │  → true → wormhole путь     │
    │     └─────────────────────────────┘           │     └─────────────────────────────┘
    │                                              │
    │  4. whClient.CreateTunnelWithCode(secret)    │  3. whClient.JoinTunnel(secret)
    │     → создаёт mailbox (nameplate из secret)  │     → подключается к mailbox
    │     → ждёт получателя                        │     → wormhole PAKE
    │                                              │
    │        ┌──── wormhole PAKE + transit ────┐   │
    │        │  зашифрованный канал установлен  │   │
    │        └─────────────────────────────────┘   │
    │                                              │
    │  5. tunnel.Serve(ctx, webdavAddr)            │  4. Остановить свой WebDAV сервер
    │     → принимает подключения через туннель    │  5. TCP listener на webdavAddr
    │     → dial локальный WebDAV                  │  6. Каждое подключение →
    │     → tunnel.Proxy(tunnelConn, webdavConn)   │     tunnel.Dial() → Proxy()
    │                                              │
    │     browser получателя ──→ tunnel ──→ WebDAV отправителя
    │                                              │
    │  7. Туннель живёт пока:                      │  7. Туннель живёт пока:
    │     - пользователь не нажмёт Cancel          │     - пользователь не нажмёт Cancel
    │     - приложение не закроется                │     - приложение не закроется
    │     - соединение не разорвётся               │     - соединение не разорвётся
    └──────────────────────────────────────────────┴──────────────────────────────────────
```

### Предварительные условия

1. **Отправитель**:
   - WebDAV сервер активен (`treeButton.Icon == VisibilityOffIcon && davServer.IsActive()`)
   - В настройках relay указан wormhole mailbox (`ws://`/`wss://`)
   - Файлы **не обязательны** — WebDAV шарит send dir, туннель пробрасывает HTTP

2. **Получатель**:
   - Введён тот же secret что у отправителя
   - В настройках relay указан wormhole mailbox

3. **Wormhole mailbox + transit relay** доступны

### Файлы не нужны

В режиме WebDAV (`treeButton.Icon == VisibilityOffIcon`) отправитель не обязан добавлять файлы:
- `seady()` возвращает `true` без файлов (пустой `fileentries.Range`)
- Блок сбора файлов пропускается (`if treeButton.Icon == theme.VisibilityIcon()` — false)
- В croc режиме отправляется заглушка `{Name: filepath.Base(os.DevNull)}`
- В wormhole режиме — вообще никаких файлов, только WebDAV туннель

### Может ли wormhole работать без передачи файла?

**Да.** Tunnel API (`CreateTunnelWithCode`/`JoinTunnel`) не имеет понятия файла или offer — в отличие от `SendText`/`Receive`. Это просто PAKE + transit → зашифрованная труба.

Поток:
1. Обе стороны: `claimNameplate` → PAKE → version → transit hints
2. Transit TCP установлен (direct или relay)
3. `rc.Close(Happy)` → **mailbox удалён, ресурсы сервера освобождены**
4. `tunnel.NewSession(readLoop)` → туннель готов

После этого transit TCP живёт пока соединение не разорвётся. Никаких файлов, никаких offer.

**Если отправитель не вызовет `tunnel.Serve()`**:
- Туннель простаивает, transit relay держит idle TCP — это нормально
- Если получатель вызовет `tunnel.Dial()` → OPEN → `handleOpen` → нет listener → CLOSE → dial отклонён

**Если отправитель вызовет `tunnel.Serve()` но WebDAV не запущен**:
- `net.Dial("tcp", webdavAddr)` внутри Serve падает → соединение через туннель закрывается
- Туннель при этом остаётся жив — получатель может повторить dial

**Сервер**: mailbox удалён после PAKE, transit relay держит TCP пока соединение активно. Простой не создаёт нагрузку.

### Рассинхронизация — не проблема

Порядок нажатия кнопок не важен. Wormhole mailbox сервер — очередь сообщений:

1. Обе стороны вызывают `claimNameplate(nameplate)` → сервер создаёт nameplate если его нет
2. Обе стороны `WritePake` → сообщения в очереди
3. Обе стороны `ReadPake` → получают PAKE друг друга

Retry не нужен — протокол wormhole решает это на уровне mailbox.

## План

### Шаг 0: go.mod — replace директива

```
replace github.com/psanford/wormhole-william => ../wormhole-william
```

Добавить в `require`: `github.com/psanford/wormhole-william v0.0.0-00010101000000-000000000000`

Выполнить `go mod tidy`.

### Шаг 1: wormhole-william — метод CreateTunnelWithCode

Текущий `CreateTunnel()` генерирует случайный nameplate и код. Нужно создать туннель с **предопределённым кодом** (secret пользователя).

Добавить в `wormhole/tunnel_integration.go`:

```go
func (c *Client) CreateTunnelWithCode(ctx context.Context, code string) (*tunnel.Tunnel, error) {
    // Аналог CreateTunnel, но:
    // 1. nameplate = nameplateFromCode(code) — извлечь из кода
    // 2. rc.AttachMailbox(ctx, nameplate) — claim конкретный nameplate
    //    (AttachMailbox уже делает claimNameplate + openMailbox — новый метод не нужен)
    // 3. PAKE + version + transit → tunnel.Session
    // 4. вернуть tunnel.Tunnel
}
```

**Не требует изменений в rendezvous** — `AttachMailbox` уже делает `claimNameplate` + `openMailbox`.

### Шаг 2: crocson — определение wormhole-режима

```go
func isWormholeRelay(addr string) bool {
    return strings.HasPrefix(addr, "ws://") || strings.HasPrefix(addr, "wss://")
}
```

Relay struct (`relays.go`) — **без изменений**:
- `Address` = `ws://mailbox.example.com:4000/v1` (wormhole mailbox URL)
- `Connect` — не используется для wormhole

Transit выводится из mailbox URL автоматически (тот же хост, порт + 1):
```
ws://relay.magic-wormhole.io:4000/v1 → relay.magic-wormhole.io:4001
```
Дефолты wormhole-william: `relay.magic-wormhole.io:4000` / `transit.magic-wormhole.io:4001`.

### Шаг 3: crocson — новый файл `wormhole_tunnel.go`

Публичный API для wormhole туннеля в crocson:

```go
type WormholeTunnel struct {
    tunnel   *wh_tunnel.Tunnel
    cancel   context.CancelFunc
    ctx      context.Context
    isSender bool
    wg       sync.WaitGroup
}

// StartWormholeSender — вызывается из mainButton (send) при wormhole relay
func StartWormholeSender(ctx context.Context, secret, mailboxURL, transitAddr, webdavAddr string) (*WormholeTunnel, error)

// StartWormholeReceiver — вызывается из mainButton (recv) при wormhole relay
func StartWormholeReceiver(ctx context.Context, secret, mailboxURL, transitAddr, webdavAddr string) (*WormholeTunnel, error)

func (wt *WormholeTunnel) Close() error
```

**StartWormholeSender**:
1. `whClient.CreateTunnelWithCode(ctx, secret)` → `*tunnel.Tunnel`
2. `tunnel.Serve(ctx, webdavAddr)` в горутине — принимает входящие соединения из туннеля, подключается к локальному WebDAV, проксирует

**StartWormholeReceiver**:
1. `whClient.JoinTunnel(ctx, secret)` → `*tunnel.Tunnel`
2. TCP listener на `webdavAddr`
3. Для каждого подключения: `tunnel.Dial(ctx, webdavAddr)` → `tunnel.Proxy(localConn, tunnelConn)`

### Шаг 4: crocson — модификация `send.go` mainButton

mainButton handler (строка 779) — добавить раннюю развилку:

```go
mainButton = widget.NewButtonWithIcon(lp("Send"), theme.MailSendIcon(), func() {
    // ... валидация entry, seady() ...

    relayAddr := a.Preferences().String("relay-address")
    
    if isWormholeRelay(relayAddr) {
        // === WORMHOLE ПУТЬ ===
        go startWormholeSend(secret, relayAddr, connectAddr, webdavAddr)
        return
    }

    // === CROC ПУТЬ (текущий код без изменений) ===
    // ... текущий код crocNew, client.Send, progress goroutine ...
})
```

**startWormholeSend** (новая функция в send.go или wormhole_tunnel.go):
1. Проверить что WebDAV активен (`davServer.IsActive()`)
2. `showCancel()` — показать Cancel, скрыть mainButton
3. `StartWormholeSender(ctx, secret, mailboxURL, transitAddr, davServer.addr)`
4. Обновить UI: topline "Connected via wormhole", WebDAV tree виден
5. Ждать `<-cancelChan` или `<-ctx.Done()` или ошибку туннеля
6. `defer: wt.Close()`, `hideCancel()`, обновить UI

**Отличия от croc пути**:
- Нет `crocNew()` / `client.Send()` / progress goroutine
- Нет ticker loop для polling `client.Step1ChannelSecured`
- Нет `client.Step2FileInfoTransferred` / file progress
- Туннель живёт до Cancel (не до завершения передачи файла)
- Файлы скачиваются через WebDAV (браузер), не через croc

### Шаг 5: crocson — модификация `recv.go` mainButton

mainButton handler (строка 592) — аналогичная развилка:

```go
mainButton = widget.NewButtonWithIcon(lp("Download"), theme.DownloadIcon(), func() {
    // ... валидация entry ...

    relayAddr := a.Preferences().String("relay-address")
    
    if isWormholeRelay(relayAddr) {
        // === WORMHOLE ПУТЬ ===
        go startWormholeRecv(secret, relayAddr, connectAddr, webdavAddr)
        return
    }

    // === CROC ПУТЬ (текущий код без изменений) ===
    // ... текущий код crocNew, client.Receive, progress goroutine ...
})
```

**startWormholeRecv**:
1. `showCancel()` 
2. `davServer.Stop()` — остановить свой WebDAV
3. `StartWormholeReceiver(ctx, secret, mailboxURL, transitAddr, davServer.addr)`
4. Обновить UI: topline "Connected via wormhole", WebDAV tree виден (удалённый)
5. `davServer.SetRemote(true)`
6. Ждать `<-cancelChan` или `<-ctx.Done()`
7. `defer: wt.Close()`, `davServer.SetRemote(false)`, `hideCancel()`, перезапуск WebDAV

### Шаг 6: webdav.go — EnableTCPForwarding и DisableTCPForwarding

**EnableTCPForwarding** — не вызывается в wormhole режиме. Туннель создаётся напрямую из mainButton через `StartWormholeSender`/`StartWormholeReceiver`.

**DisableTCPForwarding** — вызывается в defer croc-пути. В wormhole пути туннель закрывается через `wt.Close()`.

Возможная адаптация `EnableTCPForwarding`:
```go
func (s *WebDAVServer) EnableTCPForwarding(client *croc.Client) error {
    if isWormholeRelay(client.Options.RelayAddress) {
        return fmt.Errorf("wormhole mode: use StartWormholeSender/Receiver instead")
    }
    return s.enableCrocTCPForwarding(client)
}
```

### Шаг 7: UI настроек relay — без изменений

Поле `Address` в профиле relay уже может содержать `ws://...`. Поле `Connect` — для transit relay.

## Файлы

| Файл | Действие | Описание |
|------|----------|----------|
| `go.mod` | Изменить | replace + require wormhole-william |
| `wormhole_tunnel.go` | **Новый** | StartWormholeSender, StartWormholeReceiver, WormholeTunnel |
| `send.go` | Изменить | mainButton: развилка croc/wormhole, startWormholeSend |
| `recv.go` | Изменить | mainButton: развилка croc/wormhole, startWormholeRecv |
| `webdav.go` | Изменить | isWormholeRelay, защита EnableTCPForwarding в wormhole режиме |
| `relays.go` | Без изменений | Relay struct достаточен |
| `wormhole-william/wormhole/tunnel_integration.go` | Изменить | Добавить CreateTunnelWithCode |

## Открытые вопросы

1. **AppID**: `"abakum.github.io/wormhole/forward"` — изолирует туннель от других wormhole-клиентов. Разные AppID = невозможно соединиться.
   - wormhole-william: `"lothar.com/wormhole/text-or-file-xfer"` (transit, single TCP)
   - fowl: `"meejah.ca/wormhole/forward"` (Dilation, subchannels, reconnect)
   - crocson: `"abakum.github.io/wormhole/forward"` (transit, tunnel.Session)

2. **Формат secret**: `nameplateFromCode` требует числовой префикс до первого `-`. Croc секреты уже имеют формат `4-sodium-bottle-...` — работают напрямую. Если секрет без числового префикса — детерминированно выводим из SHA-256 хеша:
   ```go
   func ensureWormholeCode(secret string) string {
       parts := strings.SplitN(secret, "-", 2)
       if len(parts) >= 2 {
           if _, err := strconv.Atoi(parts[0]); err == nil {
               return secret
           }
       }
       h := sha256.Sum256([]byte(secret))
       prefix := binary.BigEndian.Uint32(h[:4]) % 10000
       return fmt.Sprintf("%d-%s", prefix, secret)
   }
   ```
   Croc: 4 слова, wormhole: 2 слова — не проблема, `nameplateFromCode` берёт только часть до первого `-`.

3. **WebDAV tree**: Одинаково для croc и wormhole режима:
   - **До соединения**: оба пира видят свою корзину (send или recv, зависит от swap)
   - **После соединения**: WebDAV сервер получателя останавливается → tunnel forwarding → получатель видит корзину отправителя через туннель
   - `switchToWebDAVTree()` + polling `/api/messages` работает так же — HTTP-запросы идут через туннель

4. **Чат и видеозвонок**: Работают через HTTP API (`/api/chat/ws`, `/api/call/*`), который проходит через WebDAV сервер → туннель → WebDAV сервер. Должно работать без изменений.
