# План: WebDAV/чат через wormhole туннель

## Идея

GUI не меняется. Когда в настройках relay указан wormhole mailbox (вместо croc relay), система автоматически использует wormhole-туннель для проксирования WebDAV/чата вместо croc TCP-форвардера. Код wormhole совпадает с croc secret.

## Ключевое решение: wormhole-william (чистый Go) вместо fowld (Python subprocess)

[`wormhole-william`](https://github.com/psanford/wormhole-william) — чистая Go-реализация magic wormhole. Она **НЕ** поддерживает Dilation (субканалы), но предоставляет:

1. **Rendezvous клиент** (`rendezvous.Client`) — подключение к mailbox серверу, PAKE key exchange (SPAKE2)
2. **Transit** — прямые или relay TCP-соединения между пирами с handshake и NaCl-шифрованием

Этого достаточно. Мы используем wormhole-william для установления зашифрованного `net.Conn` между пирами через transit relay, а поверх запускаем **существующий** `TCPForwarder` — он уже умеет мультиплексировать, шифровать и буферизировать.

```
Отправитель (crocson)                          Получатель (crocson)
    │                                                │
    │  [WebDAV :8080]                                │  [браузер :8090]
    │       │                                        │       │
    │  wormhole-william                              │  wormhole-william
    │  rendezvous → PAKE (croc secret)               │  rendezvous → PAKE (croc secret)
    │  transit → зашифрованный net.Conn              │  transit → зашифрованный net.Conn
    │       │                                        │       │
    │  TCPForwarder (существующий)                    │  TCPForwarder (существующий)
    │       │                                        │       │
    │       └──── transit relay / direct TCP ────────┘
    │                                                │
    │  browser → :8090 → TCPForwarder → wormhole → TCPForwarder → :8080 → WebDAV
```

**Преимущества перед fowld subprocess**:
- Нет зависимости от Python
- Нет subprocess-мангемента (stdin/stdout JSON)
- Единый бинарник
- Переиспользуем `TCPForwarder` без изменений

## Детектирование типа relay

```go
func isWormholeRelay(addr string) bool {
    return strings.HasPrefix(addr, "ws://") || strings.HasPrefix(addr, "wss://")
}
```

Это позволяет использовать существующий UI relay-профиля без изменений.

## План реализации

### 1. Зависимость: `go get github.com/psanford/wormhole-william`

### 2. Новый файл: `wormhole.go` — wormhole-туннель через transit

```go
// WormholeTunnel устанавливает зашифрованное TCP-соединение между пирами
// через wormhole rendezvous + transit, используя croc secret как код.
type WormholeTunnel struct {
    conn         net.Conn       // зашифрованное transit-соединение
    cancel       context.CancelFunc
    active       bool
    mu           sync.RWMutex
}
```

**Методы**:

- `ConnectSender(ctx, code, mailboxURL, transitRelay string, localServerAddr string) (*TCPForwarder, error)`
  1. Создаёт `wormhole.Client{RendezvousURL: mailboxURL, TransitRelayAddress: transitRelay}`
  2. Вызывает `SendText(ctx, "", wormhole.WithCode(code))` — отправляет пустое текстовое сообщение чтобы инициировать PAKE + transit
  3. После установления transit-соединения получает `net.Conn`
  4. Создаёт `TCPForwarder([]net.Conn{conn}, true, nil, localServerAddr)` — использует transit-соединение как транспорт для существующего TCPForwarder

- `ConnectReceiver(ctx, code, mailboxURL, transitRelay string, listenAddr string) (*TCPForwarder, error)`
  1. Создаёт `wormhole.Client{...}`
  2. Вызывает `Receive(ctx, code)` — получает входящее соединение
  3. Transit даёт `net.Conn`
  4. Создаёт `TCPForwarder([]net.Conn{conn}, false, nil, "")` + слушает на `listenAddr`

**Проблема**: wormhole-william не отдаёт `net.Conn` напрямую — его transit инкапсулирован в `transportCryptor` (записи с length-prefix + NaCl encryption). Нужен обходной путь.

#### Вариант A: Два соединения через SendText/Receive

wormhole-william использует одно transit-соединение для каждой передачи. Нам нужно 2 (как в croc-туннеле — одно для управляющих сообщений, другое для данных). Решение:

1. Отправитель делает `SendText` с кодом `croc-secret-tunnel-1` → transit conn 1
2. Отправитель делает `SendText` с кодом `croc-secret-tunnel-2` → transit conn 2
3. Получатель делает `Receive(croc-secret-tunnel-1)` → transit conn 1
4. Получатель делает `Receive(croc-secret-tunnel-2)` → transit conn 2

Но wormhole-william не отдаёт сырой `net.Conn` — он возвращает `transportCryptor` (записи с encryption поверх conn).

#### Вариант B: Использовать transit напрямую, в обход SendText/Receive

Скопировать `fileTransport` из wormhole-william и адаптировать:
1. Использовать `rendezvous.Client` для PAKE key exchange
2. Получить `transitKey` из shared key
3. Сделать `newFileTransport(transitKey, relayAddr)` → получить `net.Conn`
4. Этот `net.Conn` — зашифрованный транспорт, поверх которого работает `TCPForwarder`

Это наиболее практичный путь. Код `file_transport.go` в wormhole-william (~640 строк) можно форкнуть и адаптировать.

#### Вариант C (рекомендуемый): Обёртка io.ReadWriteCloser → net.Conn

wormhole-william возвращает данные через `IncomingMessage` (io.Reader) и отправляет через внутренний `transportCryptor`. Можно:

1. Создать `wormhole.Client` на каждой стороне
2. Использовать `SendText`/`Receive` для установления PAKE и transit
3. Обернуть в `io.Pipe` или `net.Pipe` чтобы получить `net.Conn`-совместимый интерфейс
4. Подключить к существующему `TCPForwarder`

Или ещё проще — **не использовать TCPForwarder поверх wormhole**, а вместо этого использовать wormhole-william для signalling (обмен IP-адресами), а потом установить прямое TCP-соединение и использовать TCPForwarder как сейчас.

### Рекомендуемый подход: Вариант B (адаптированный transit)

**Новый файл `wormhole_transport.go`** — адаптация transit из wormhole-william:

1. Используем `rendezvous.Client` для PAKE key exchange с croc secret как кодом
2. Получаем `transitKey` = `deriveTransitKey(sharedKey, appID)`
3. Выполняем transit handshake: прямые подключения + relay fallback
4. Получаем **сырой** `net.Conn` (до encryption) — transit relay просто пробрасывает TCP
5. Используем этот `net.Conn` как транспорт для существующего `TCPForwarder`

Шифрование обеспечивается на уровне `TCPForwarder` (существующий `crypt.Encrypt/Decrypt` с croc key) ИЛИ на уровне transit (`transportCryptor` с NaCl). Можно использовать оба уровня или отключить один.

**Упрощение**: можно использовать transit relay как «глупый» TCP relay (как croc relay), просто пересылающий байты. Тогда transit encryption не нужен — TCPForwarder сам шифрует.

### 3. Модификация: `webdav.go` — диспетчеризация

```go
func (s *WebDAVServer) EnableTCPForwarding(client *croc.Client) error {
    if isWormholeRelay(client.Options.RelayAddress) {
        return s.EnableWormholeForwarding(client)
    }
    return s.enableCrocTCPForwarding(client) // переименованный текущий метод
}

func (s *WebDAVServer) EnableWormholeForwarding(client *croc.Client) error {
    // 1. Установить wormhole transit-соединение
    // 2. Создать TCPForwarder поверх net.Conn
    // 3. Дальше — как в croc-туннеле
}
```

### 4. Модификация: `send.go` / `recv.go` — без изменений

Точки активации (`send.go:1097`, `recv.go:763`) не меняются:
```go
if client.Step1ChannelSecured {
    if davServer.IsActive() {
        if !davServer.IsTCPForwardingActive() {
            davServer.EnableTCPForwarding(client) // сам выберет croc или wormhole
        }
    }
}
```

### 5. Настройки: `relays.go`

Детектирование по формату адреса:
```go
func isWormholeRelay(addr string) bool {
    return strings.HasPrefix(addr, "ws://") || strings.HasPrefix(addr, "wss://")
}
```

Relay struct не меняется — `Address` может быть как `host:port` (croc), так и `wss://...` (wormhole mailbox).

## Отличия от croc-туннеля

| Аспект | croc туннель | wormhole туннель |
|--------|-------------|-----------------|
| Транспорт | 2 TCP к croc relay | transit (direct + relay) |
| Шифрование туннеля | AES (TCPForwarder) | NaCl (transit) + AES (TCPForwarder) |
| Мультиплексирование | TCPForwarder (connID) | TCPForwarder (connID) |
| Реконнект | Нет | Нет (transit не reconnect) |
| Зависимость | croc (Go, встроена) | wormhole-william (Go) |
| Код | croc secret | croc secret |
| Relay | croc relay | wormhole transit relay |
| Signalling | croc room | wormhole mailbox + PAKE |

## Файлы для создания/изменения

| Файл | Действие | Описание |
|------|----------|----------|
| `go.mod` | Изменить | Добавить `github.com/psanford/wormhole-william` |
| `wormhole_transport.go` | Новый | Адаптированный transit для установления `net.Conn` |
| `webdav.go` | Изменить | Добавить `EnableWormholeForwarding`, диспетчеризацию |
| `send.go` / `recv.go` | Без изменений | Точки активации не меняются |
| `relays.go` | Без изменений | `isWormholeRelay()` детектирует по формату |

## Вопросы

1. **Transit encryption**: TCPForwarder уже шифрует данные. Нужен ли второй уровень шифрования через `transportCryptor` (NaCl)? Или использовать transit relay как «глупый» TCP-прокси?

2. **Количество transit-соединений**: Текущий TCPForwarder использует 2 канала (conns[0] для control, conns[1] для data). Нужно ли 2 отдельных transit-соединения или можно мультиплексировать всё через одно?

3. **AppID**: wormhole-william использует `WormholeCLIAppID = "lothar.com/wormhole/text-or-file-xfer"`. Для crocson-туннеля нужен свой AppID (например `"com.github.abakum.crocson/tunnel"`), чтобы не конфликтовать с обычными wormhole-клиентами.
