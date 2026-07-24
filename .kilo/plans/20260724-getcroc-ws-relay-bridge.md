# Маршрут crocson до relay через WebSocket-мост getcroc.com

> Статус: **checkpoint (разведка)**. Реализация и детализация dial-точки в crocson — на следующую сессию.

## Контекст / проблема

- crocson — нативный Go (Fyne/Android через cgo). Сейчас ходит к relay напрямую по TCP.
- `croc.schollz.com` (публичный relay croc) **недоступен**.
- `getcroc.com` (там крутится `croc serve` → пакет `webrelay`) — **доступен**, и является мостом к тому же relay.
- Цель: пустить трафик crocson → `getcroc.com/ws` → `croc.schollz.com`, не переписывая протокол croc.

## Верифицированная архитектура (коммит schollz/croc `1f34c726`)

### 1. Мост `webrelay` — байт-прозрачный WS↔TCP
`src/webrelay/webrelay.go`:
- `GET /ws?port=<port>` → `net.Dialer.DialContext("tcp", relayHost:port)` и дальше `io.CopyBuffer` в обе стороны через `websocket.NetConn`.
- Комментарий в пакете: мост **«never participates in the croc protocol; only forwards an opaque byte stream»**.
- Allowlist портов по умолчанию: `9009, 9010–9017`. Порт вне списка → 403.
- `runtimeConfig.RelayAddress = croc.schollz.com:9009` (первый порт) отдаётся в `/config.js` → браузерный контрольный коннект идёт на `9009`.
- Origin: `websocket.AcceptOptions{OriginPatterns}`. Проверка срабатывает **только при наличии заголовка Origin**. Браузеры его шлют, нативный клиент (crocson) — нет → проходит без оркестрации Origin.

### 2. Кадрирование — нативный формат croc, НЕ веб-специфика
`src/comm/comm.go`:
- `MAGIC_BYTES = []byte("croc")`.
- `Comm.Write(b)`: `MAGIC(4) + uint32 LE len(4) + payload` — одним `connection.Write`.
- `Comm.Read()`: `io.ReadFull` 4 байта magic → 4 байта len → `len` байт payload.
- Это формат **всех** croc-соединений (CLI-пир ↔ relay, и между пирами через `pipe()`).

`web/src/protocol/framing.ts` (`encodeFrame`/`FrameDecoder`) — это **TS-порт** того же самого кадра: magic `[0x63,0x72,0x6f,0x63]` ("croc") + `setUint32(4, len, true)` + payload. Полностью совпадает с `comm.go`.

**Вывод:** `wss://getcroc.com/ws?port=9009` функционально эквивалентен TCP-сокету до `croc.schollz.com:9009`. Дополнительно кадрировать ничего не нужно — croc это уже делает сам.

### 3. Relay — рукопожатие + тупой pipe
`src/tcp/tcp.go` (`server.clientCommunication`, `pipe`):
- `weakKey = []byte{1,2,3}`; PAKE `siec` со стороны relay (`InitCurve(weakKey, 1, "siec")`).
- Клиент (`tcp.ConnectToTCPServer`): `InitCurve(weakKey, 0, "siec")`, обмен PAKE, `crypt.New(strongKey, salt)`, шлёт зашифрованный пароль relay, получает `banner + "|||" + ip`, шлёт комнату, ждёт `"ok"`.
- После спаривания двух пиров в комнате — `pipe(conn1, conn2)` копирует **сырые байты** между ними без разбора (кадры текут end-to-end нетронутыми).

### 4. Веб-клиент как эталон
`web/src/protocol/client.ts` (`connectRelay`, `sendFiles`, `receiveFiles`):
- Контрольный порт = `controlPort(relayAddress)` → `9009`; дата-порты = `dataPorts(banner)` (из ответа relay).
- Каждое croc-соединение — отдельный WS к `/ws?port=<port>` (`transport.ts` `CrocSocket.connect`).
- Реализует весь протокол croc поверх WS (PAKE relay, room, встречный PAKE p256, `fileinfo`, чанки 32 KiB).

## Ключевой вывод / направление реализации

Не «реимплементировать croc поверх WS», а **заменить только транспорт**:

1. Реализовать `net.Conn`, у которого `Read`/`Write` ходят в WebSocket `wss://getcroc.com/ws?port=<port>` (чистый байтовый поток, **без** собственного кадрирования — кадры ставит `comm.Comm`).
2. Скармливать его в `comm.New(conn)` / логику `tcp.ConnectToTCPServer` (или точку dial в crocson). Весь протокол (PAKE `siec`, пароль, комната, дата-каналы) остаётся от croc как есть.
3. Каждое croc-соединение = один WS: контроль `9009` + по одному WS на каждый дата-порт из баннера (все в allowlist `9009–9017`).

Важный нюанс для адаптера: `comm.Write` пишет весь кадр **одним** `conn.Write`, поэтому естественно `один кадр = одно WS-сообщение`. На `Read` нужно отдавать contiguous байтовый поток (конкатенировать приходящие WS-сообщения в буфер) — `comm.Read` через `io.ReadFull` сам доберёт нужное число байт.

## Открытые вопросы / следующие шаги (на след. сессию)

- [ ] Найти точку dial в crocson: как сейчас задаётся relay и где открывается TCP (`send.go`, модели/опции croc). Определить, где переопределить `net.Dial` на WS-dial.
- [ ] Выбрать Go WS-клиент: `github.com/coder/websocket` (та же либа, что на сервере → `NetConn` можно переиспользовать напрямую) vs `gorilla/websocket`. Предпочтение — `coder/websocket` (есть готовый `NetConn`).
- [ ] Решить, как пробрасывать порт в URL: crocson знает запрошенный порт (контроль vs дата) → `?port=N`.
- [ ] Несколько одновременных WS (контроль + N дата): жизненный цикл, таймауты, закрытие.
- [ ] Конфиг-тогл: прямой TCP vs WS-мост (на случай, если relay снова станет доступен / для не-RU).
- [ ] Поведение при недоступности getcroc.com / ошибках моста (502 «relay is unavailable» из `webrelay.go`).

## Валидация

- end-to-end: send/receive между crocson (через `getcroc.com/ws`) и CLI-пиром на `croc.schollz.com`, в т.ч. многопортовая передача (контроль + дата-каналы).
- Проверить, что кадр `comm.Comm` доходит нетронутым (сравнить с эталонным веб-клиентом).
- Проверить из сети, где `croc.schollz.com` заблокирован, а `getcroc.com` доступен.

## Out of scope (пока)

- Сама реализация WS→`net.Conn` и правки dial в crocson (следующая сессия).
- Поддержка браузерного клиента crocson.
- Альтернативные обходы (свой relay и т.п.) — отдельная тема.

## Ссылки

- Коммит: https://github.com/schollz/croc/commit/1f34c726c11c4c886f210ddd5461fd79dc202ff6
- `src/webrelay/webrelay.go` — мост `/ws`
- `src/comm/comm.go` — кадрирование `MAGIC "croc"`
- `src/tcp/tcp.go` — relay: weakKey/PAKE/`pipe`
- `src/cli/cli.go` — команда `croc serve` (флаги `--bind/--relay/--ports`)
- `web/src/protocol/{client,transport,framing}.ts` — эталонный веб-клиент
