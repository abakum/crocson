# Маршрут crocson до relay через WebSocket-мост getcroc.com

> Статус: **checkpoint (разведка)**. Реализация и детализация dial-точки в crocson — на следующую сессию.
>
> ✅ **Верификация актуальности 2026-08-05**: план подтверждён — и сетевыми пробами, и против
> текущего кода `orig/croc`. См. блок «Верификация 2026-08-05» в конце.
> Вопросы dial-точки и выбора WS-либы — **закрыты** (отмечены `[x]` ниже).
>
> 🏗️ **Архитектура (главное):** **ноль правок в форке** `abakCroc/croc` + **ноль новых зависимостей**.
> WS-dial вклинивается через существующий прокси-реестр croc (`proxy.RegisterDialerType` + `comm.Socks5Proxy`),
> всё переключение и WS-код — в crocson (`wsbridge.go` + `def()`). Тогл — префикс `wss:` в поле `socks5`
> (не `connect` — т.к. мост перехватывает именно `comm.Socks5Proxy`). См. «Решение дизайна: `wss:`».
>
> 🏁 **ФИНАЛЬНЫЙ ИТОГ (2026-08-06):** мост реализован, корректен и **валидирован на bulk-данных**
> (косвенно: `socks5://vps` через ту же прокси-ветку `comm.NewConnection→dialer.Dial` несёт 121 MB на
> 4.7 MB/s). **НО публичный `getcroc.com` DPI-троттлится из РФ для bulk** → `wss:`→getcroc из РФ
> непригоден для больших файлов (handshake проходит, данные стопорятся). **Рабочее RU-решение =
> `socks5` = личный VPS** (доказано). `socks5` и `wss:` взаимоисключающи (одно поле/глобаль).
> Решение по судьбе `wsbridge.go` — см. «Открытое решение» в конце.

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
- `runtimeConfig.RelayAddress = <RelayHost>:<AllowedPorts[0]>` (первый порт) отдаётся в `/config.js` → браузерный контрольный коннект идёт на `9009`. **RelayHost — это backend, к которому webrelay сам дозванивается сервер-сайд** (`net.Dial tcp`); он задаётся флагом `--relay` при `croc serve`, а НЕ хардкодом. На опорном коммите `1f34c726` дефолт был `ipv4.getcroc.com`, но **развёрнутый getcroc.com флаг перебивает** (см. ниже «Дрейф upstream»/`disco.json`). Для crocson это непрозрачно: ему достаточно `wss://getcroc.com/ws?port=N`, байты текут через серверную сторону.
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

## Решение дизайна: `wss:` в поле `socks5` (2026-08-05)

**Текущее состояние поля `socks5` (проверено в коде, 2026-08-05):**
- Существует в UI (`settings.go:136,427`, form-item «socks5», preference `"socks5"`).
- **Используется**: в `def()` (`send.go:2958` `s = defs(socks5, a.Preferences().String("socks5"))`)
  → возвращается как `s` → выставляется в глобальную `comm.Socks5Proxy` (`send.go:1368-1369`
  `... comm.Socks5Proxy, comm.HttpProxy = def(a)`). Пакет `comm` — `github.com/schollz/croc/v10/src/comm`;
  croc-диалер читает `comm.Socks5Proxy` внутри `comm.NewConnection` (comm.go:44-63) — **это и есть
  та самая глобальная, которую перехватывает мост**, поэтому триггер `wss:` логичнее здесь, а не в `connect`.
- **Не копируется в диплинк/профиль**: заполнение `relay.Socks5` и чтение в deeplink закомментированы
  (`relays.go:49`, `applinks.go:564`, `recv.go:156`). Поле `Socks5` есть в JSON-схеме релея
  (`relays.go:70`, тег `json:"socks4"`), но не заполняется — режим WS пока не переживёт share-по-коду
  (out of scope). Поле `connect` (`comm.HttpProxy`) в этом дизайне **не трогается вообще**.

**Решение (директива пользователя, уточнено 2026-08-05):** префикс `wss:` в поле `socks5` считать
сокращением полного адреса моста `wss://getcroc.com/ws?port=N` и **признаком работы через мост**.
Хост `getcroc.com` захардкожен в сокращении; `N` — порт соединения. Выбор поля `socks5` (а не `connect`)
обусловлен тем, что мост перехватывает именно `comm.Socks5Proxy`, в которое маппится `socks5` —
значение течёт в «свою» глобальную без кросс-связывания.

**Семантика порта (важная поправка к «первый порт из поля ports»):** croc дозванивается отдельно
на контроль и на каждый дата-канал (`tcp.ConnectToTCPServer` вызывается в `croc.go:1151/1355/1472/1705/1846/2371`,
`address` = `host:port` с **разным портом**). Поэтому:
- **контрольное/meeting-соединение** → порт = первый из поля `ports` (как и сказал пользователь);
  `send.go` `ensurePort` уже вшивает его в `RelayAddress`.
- **дата-каналы** → порты приходят из баннера relay; WS-диалер берёт порт из `address` каждого dial'а.
Если бы все соединения ушли на `?port=9009`, relay не размечал бы каналы → передача сломалась бы.
**Вывод:** порт для URL всегда извлекается из `address` очередного dial'а, а не хардкодом.

**Точка перехвата = БЕЗ правок в форке, через существующий прокси-реестр croc.**
В `comm.NewConnection` уже есть прокси-ветка (`orig/croc` `comm.go:44-63`): при
`Socks5Proxy != ""` и не-local `address` croc зовёт `proxy.FromURL(url, proxy.Direct)` из
`golang.org/x/net/proxy` — а это **реестр диалеров по scheme** (`RegisterDialerType`,
подтв. в кеше модулей `proxy.go:72,81`). Дальше `dialer.Dial("tcp", address)` (строка 63) и
результат уходит в `comm.New` (строка 94). Поэтому crocson просто **регистрирует свой диалер
для scheme `wss`** — и croc сам гонит через него весь трафик, не зная про WS:
- `proxy.RegisterDialerType("wss", wsFactory)` — один раз (`sync.Once`).
- `wsFactory(u *url.URL, forward proxy.Dialer)` возвращает диалер, чей `Dial(network, address)`:
  порт из `address` (`host:port`) + база моста из `u` (`host=getcroc.com`, `path=/ws`) →
  `wss://getcroc.com/ws?port=<port>`, дозвон `websocket.Dial` и обёртка `websocket.NetConn`
  (`MessageBinary`) → `net.Conn`.
- croc: `dialer.Dial("tcp", address)` → `comm.New(connection)`. Весь протокол (PAKE `siec`,
  пароль, комната) нетронут.

**Бонус — ноль новых зависимостей:** `golang.org/x/net/proxy` уже в графе (через `croc/comm`);
`nhooyr.io/websocket v1.8.17` уже в `go.sum` (indirect, у него есть готовый `NetConn`) —
переносим в `direct`, версию не трогаем. `gorilla/websocket` для моста не нужен (остаётся для иных целей).

**Переключение — целиком в crocson `def()` (send.go:2950, используется и recv):** если preference
`socks5` (значение `s`) начинается с `wss:`:
1. зарегистрировать `wss`-диалер (`sync.Once`, идемпотентно);
2. `s = "wss://getcroc.com/ws"` (перезаписать базой моста) — далее по обычному пути `comm.Socks5Proxy = s`
   активирует прокси-ветку `comm.NewConnection`;
3. `r = "getcroc.com"` (до `ensurePort`, который вшьёт meeting-порт из `ports`) — так `address`
   каждого dial'а несёт корректный порт.
`comm.HttpProxy` не трогаем (остаётся от `connect`, по умолчанию пусто). Иначе (в `socks5` реальный
socks5-URL или пусто) — прежнее поведение, прямой TCP/socks5. Обратно совместимо. **Форк не трогается.**

**Задачи реализации (всё в crocson):**
1. Новый файл `wsbridge.go`: тип `wsBridgeDialer{base *url.URL}` с `Dial(network, addr)`
   через `nhooyr.io/websocket.Dial` + `NetConn` (`MessageBinary`), свой таймаут/ретрай
   (транзитные обрывы Ростелекома); регистрация `proxy.RegisterDialerType("wss", ...)` в `sync.Once`.
2. `def()` (send.go:2950): детект `strings.HasPrefix(s, "wss:")` → п. 1–3 выше (`s` — это `socks5`).
3. UI (`settings.go:427`): placeholder/hint поля `socks5`: «`wss:` — через мост getcroc.com
   (иначе socks5://host:port)».
4. (future) Реактивировать копирование `socks5` в диплинк (`relays.go:49`, `applinks.go:564`),
   чтобы режим моста доживал до получателя по коду.

**Риски/edge-cases:**
- Ветка `comm.Socks5Proxy` занята мостом → **SOCKS5-прокси и WS-мост взаимоисключающи** (приемлемо).
- `force-local` / local-IP-адрес корректно обходят мост: `comm.NewConnection` при `IsLocalIP(address)`
  идёт в direct-TCP (комментарий `comm.go:44,64`).
- getcroc.com недоступен / мост вернул `502 relay is unavailable` → понятная ошибка + ретрай
  (croc сам ретраит `ConnectToTCPServer`; WS-dial тоже ретраит).
- `proxy.Dialer.Dial` не несёт `tlimit` → WS-dial использует свой таймаут.
- IPv6 (`RelayAddress6`): в режиме моста должен указывать на getcroc.com (или отключаться).

## Открытые вопросы / следующие шаги (на след. сессию)

- [x] **Точка dial в crocson (найдена 2026-08-05, подход пересмотрен — БЕЗ правок форка):** relay
  задаётся через `croc.Options.RelayAddress` (собирается в `send.go:1344`, `recv.go:1048`; дефолты
  `DEFAULT_RELAY/DEFAULT_RELAY6` в `settings.go:646-650`); фактический dial — `tcp.ConnectToTCPServer`
  → `comm.NewConnection` (`webdav.go:659` и внутри croc). crocson на форке `abakCroc/croc`, но править
  его **не нужно**: `comm.NewConnection` уже имеет прокси-ветку с реестром диалеров
  (`proxy.FromURL`/`RegisterDialerType`), через которую crocson и вклинивает WS-dial. См. «Решение дизайна».
- [x] **Go WS-клиент (уточнено 2026-08-05):** WS-адаптер живёт **в crocson** (не в форке). Используем
  `nhooyr.io/websocket v1.8.17` — уже в `go.sum` (indirect), у него есть готовый `websocket.NetConn`
  → `net.Conn` без ручного кадрирования (одно WS-сообщение = один кадр `comm.Comm`, т.к. `comm.Write`
  пишет кадр одним `conn.Write`; на `Read` `io.ReadFull` сам добирает байты). **Ноль новых зависимостей.**
  `gorilla/websocket` для моста не нужен (остаётся для иных целей).
- [x] **Проброс порта в URL (решено 2026-08-05):** порт берётся **из каждого dial-`address`**
  (`host:port`), не хардкодом — контроль = meeting-порт (первый из `ports`), дата = порты из баннера
  relay. Иначе все каналы ушли бы на `?port=9009` и relay не размечал бы их. Детали — в «Решение дизайна».
- [x] **Конфиг-тогл TCP/WS (решено 2026-08-05):** тогл = префикс в поле **`socks5`** (не `connect` —
  мост перехватывает `comm.Socks5Proxy`, в которое маппится именно `socks5`). `wss:` → режим моста;
  socks5-URL/пусто → прежний прямой TCP/socks5. Обратно совместимо; `connect`/`HttpProxy` не трогается.
- [x] **Несколько одновременных WS:** жизненным циклом управляет croc (по `ConnectToTCPServer` на канал);
  каждое → отдельный WS до `/ws?port=<port>`. Таймауты/закрытие — croc + `net.Conn` от `nhooyr/websocket`;
  нужен ретрай при транзитных обрывах (~1/5 на Ростелекоме, см. «Сеть из РФ»).
- [ ] Поведение при недоступности getcroc.com / ошибках моста (`502 relay is unavailable`,
  таймаут WS-handshake): понятная ошибка пользователю + ретрай.

## Валидация

- [x] **Проверить из сети, где `croc.schollz.com` заблокирован, а `getcroc.com` доступен — ВЫПОЛНЕНО
  2026-08-05** (живая проверка из РФ, Ростелеком Ростовская обл.; см. «Сеть из РФ» ниже): мост
  полностью рабочий (WS `101` на 9009/9010/9017), `croc.schollz.com` заблокирован.
- [~] **end-to-end (частично, 2026-08-05):** реализовано и собрано (`make arm64 wsl` ок). Живой тест
  `socks5=wss:` подтвердил, что **контрольное соединение к relay идёт через мост** — лог:
  `comm.go:56 dialing with dialer.Dial` → `comm.go:89 connected to 'getcroc.com:9009'`, далее полный
  relay-хендшейк через мост (PAKE `siec` → password → room → banner `9010..9013` → `all set`). Файл
  115.7 MB передан успешно. **НО:** в этом тесте оба пира были на одной машине/LAN, croc переключился
  на **локальный** транспорт (`TEST FLAG ENABLED, TESTING LOCAL IPS`, `local connection established to
  192.168.0.127:9009`, чанки по `127.0.0.1`/`192.168.0.127` на 127 MB/s). Значит **дата-каналы через
  мост empirically ещё не проверены** (механизм идентичен контрольному — тот же `wsBridgeDialer` на
  порты 9010–9013, уверенность высокая, но требует отдельного теста).
- [ ] **Финальный тест:** два пира в **разных сетях** (или с отключённым local-discovery /
  `force-local` выкл и без общего LAN), чтобы данные реально пошли через `getcroc.com/ws?port=9010..`.
- [ ] Проверить, что кадр `comm.Comm` доходит нетронутым (сравнить с эталонным веб-клиентом).

## Follow-up после первой реализации (2026-08-05)

- [x] **[баг, косметика] `error closing connection: failed to close WebSocket: use of closed network
  connection`** — **ИСПРАВЛЕНО 2026-08-05.** Причина: `bridgeConn.Close()` вызывал и
  `c.ws.Close(StatusNormalClosure,"")`, и `c.Conn.Close()` — но `nhooyr NetConn.Close()` (netconn.go:127)
  сам закрывает WS → двойное закрытие. **Фикс применён:** убраны поле `ws *websocket.Conn` и явный
  `c.ws.Close(...)`; `Close` теперь только `c.cancel(); return c.Conn.Close()` (контекст-родитель
  отменяется, WS закрывает сам NetConn). `go vet` чист, `make wsl`/`make arm64` ок.
- (опц.) Подтвердить, что croc не вызывает `Close` дважды на одном conn (иначе нужен `sync.Once`),
  хотя для plain TCP поведение аналогичное. В наблюдаемом логе варнинг был именно от нашего двойного
  close — после фикса должен исчезнуть.

## Второй раунд тестов (2026-08-05 ~17:00) — дата-путь через мост ПАДАЕТ (открыто)

Три прогона в одном логе:
1. **wss + LAN** (16:50): мост только для хендшейка, данные по LAN — OK (тот же лог, что и раньше;
   ещё на пре-фикс бинарнике, виден старый варнинг двойного close).
2. **Прямой TCP к `croc.schollz.com:9009`** (16:58): `croc.schollz.com` теперь резолвится в
   `142.132.189.179` (= IP getcroc.com). `comm.go:80` (НЕ мост). Дата-каналы 9010-9017 соединились,
   но на данных — `consecutive read error: ...142.132.189.179:901x: i/o timeout` → panic
   `connection forcibly closed`. **Прямой TCP упал так же.**
3. **wss (post-fix, no-local)** (17:03): 8 дата-каналов через `dialer.Dial` к getcroc.com:9010-9017,
   все соединились + PAKE + room-0..7 спарены. Варнинг двойного close **отсутствует** (фикс подтверждён ✓).
   `connected as 10.0.103.4:... -> 10.0.103.4:...` (та же машина, hairpin). После `fileinfo` (17:03:34) —
   тишина, **`start sending data!`/`beginning sending comms` не печатаются** (в отличие от LAN-прогона);
   через ~19с — `read tcp ...->142.132.189.179:443: wsarecv: ... did not properly respond` (WSAETIMEDOUT)
   → передача прервана.

**Ключевой вывод:** дата-путь падает **одинаково и для WS-моста, и для прямого TCP** к 142.132.189.179,
при этом relay-хендшейк (контроль + установка каналов + PAKE + room) проходит, а LAN работает.
Прямой TCP вообще не касается `wsbridge.go` → **почти наверняка НЕ баг нашего WS-кода**, а слой
relay/канала/DPI. Хендшейк (мелкие шифр-кадры) проходит, bulk-чанки 32 KiB стопорятся → Windows
~21с TCP-таймаут.

**Гипотезы (по убыванию правдоподобия):**
1. **DPI РК** душит устойчивый bulk-трафик к 142.132.189.179 (и :443 WS, и :9010 raw); хендшейк
   проходит, т.к. мелкий.
2. У getcroc relay лимит throughput/idle на дата-каналах (но веб-клиент же работает…).
3. **Hairpin-артефакт**: оба пира — одна машина (10.0.0.2 / 10.0.103.4), данные гоняются
   RU→getcroc→RU дважды; + 8 параллельных каналов повышают шанс падения одного.
4. [низкая] Нет WS-keepalive (nhooyr не шлёт ping) → посредник (Caddy/NAT) мог бы убить idle-WS,
   но таймаут ~19с — маловато для idle-таймаута, и падение на активных данных, не на idle.

**Решающий разделитель (ОДИН тест):**.transfer через **браузерный веб-клиент getcroc.com** с этой же
РУ-сети прямо сейчас. Браузер использует тот же `/ws`→relay путь. Если веб-клиент тоже падает на
данных → DPI/relay (мост тут не лечит). Если веб-клиент работает → наш мост чем-то отличается от
браузера и надо копать (keepalive, число каналов, framing).

**План диагностики/фикса (порядок):**
1. **Базлайн браузером** (см. выше) — разделит DPI/relay vs наш мост.
2. **Не-hairpin тест**: два РАЗНЫХ устройства/сети (или crocson ↔ CLI-пир на другой машине).
3. Изолировать нагрузку: маленький файл + `disable-multiplexing` (1 канал) — работает ли 1 канал,
   когда 8 падают.
4. (код-митигация) Добавить WS-keepalive: в `wsBridgeDialer.Dial` стартовать горутину с
   `conn.Ping(ctx)` раз в ~15-20с (nhooyr поддерживает) — собьёт idle-таймаут посредника, если
   причина в нём. Дёшево, стоит попробовать в любом случае.
5. Посмотреть, какая сторона стопорится: sender read-timeout → receiver не шлёт `recipientready`/ack
   (так и есть в логе 17:03 — `start sending data!` не дошёл).

### РЕШАЮЩЕЕ ПОДТВЕРЖДЕНИЕ (2026-08-05 17:xx): это DPI-троттлинг bulk к `142.132.189.179`, НЕ баг моста

Прямые замеры с той же РУ-сети (Ростелеком):

| Хост | Размер | Скорость | Время |
|---|---|---|---|
| `speed.cloudflare.com` 1 MB | 1 000 000 B | **1.8 MB/s** | 0.54с |
| `raw.githubusercontent.com` | 3.8 KB | 8 KB/s | 0.47с |
| **`getcroc.com/` (html)** | 11–19 KB | **635 B/s** | 30с (timeout) |
| `getcroc.com/healthz` | 3 B | — | 0.33с |

- Сеть умеет bulk (cloudflare 1.8 MB/s) → это **НЕ общая медленность**.
- `getcroc.com` мелкий (`healthz`) — мгновенно; bulk (даже 11 KB html) — ~600 B/s и stall.
- **`croc.schollz.com` теперь резолвится в тот же IP `142.132.189.179`** (коммит «use croc.schollz.com
  temporarily» = schollz перенёс public relay на инфраструктуру getcroc).

**Вывод:** DPI РК точечно троттлит устоявшийся bulk-трафик к `142.132.189.179` (host теперь общий у
getcroc.com и croc.schollz.com). Мелкий croc-handshake проходит, bulk-чанки 32 KiB режутся до
нечего → Windows ~21с timeout. Этим объясняется всё разом: crocson-handshake OK, данные stall;
прямой TCP к croc.schollz.com:9010 (тот же IP) — так же; браузер «не открывает» getcroc
(HTML+бандл не докачиваются).

**Код моста (`wsbridge.go`) КОРРЕКТЕН и не виноват** — handshake, framing, мультиканал доказаны
(8 дата-каналов через `dialer.Dial` установились, PAKE/room прошли). Premise плана («getcroc.com
доступен из РФ») верна для связности, но **ЛОЖНА для полезной пропускной способности**: relay-хост
DPI-троттлится, поэтому реальные передачи не идут **при любом коде моста**.

**Импликация (новый фокус):** проблема не транспортная, а сетевая/DPI. Мост готов, но ему нужен
**нетроттлимый путь до relay**. Варианты (отдельная тема, не мост):
- **CDN-fronting / domain fronting**: relay за Cloudflare-подобным CDN; клиент стучится на IP CDN
  (не 142.132.189.179) → CDN форвардит на origin. Эвансы IP-based троттлинга (нужен SNI-vs-IP тест:
  handshake к getcroc быстрый → троттлинг flow/IP-based, не SNI — значит fronting через CDN-IP сработает).
- **Свой relay** на нетроттлимом хосте (не 142.132.189.179) — out-of-scope пункта «свой relay».
- **ECH / no-SNI / прокси** — если окажется SNI-компонент.
- Принять, что getcroc с Ростелекома неприменим для bulk; мост остаётся полезным там, где getcroc не троттлится.

**Открытый подпункт для следующей итерации:** отличить IP-based от SNI-based троттлинг (bulk к
142.132.189.179 с чужим SNI/через fronting-IP) — определит, Enough ли CDN-fronting.

## Третий раунд (2026-08-06): `socks5` через личный VPS — РАБОТАЕТ (валидация моста + RU-решение)

Конфиг: `socks5 = socks5://<свой-VPS>` (НЕ `wss:`). crocson → VPS (SOCKS5) → `croc.schollz.com:9009`
(= 142.132.189.179). Все 8 дата-каналов через `comm.go:56 dialing with dialer.Dial` (SOCKS5-диалер).
Результат: **`goreleaser.exe 100% (121/121 MB, 4.7 MB/s) [32s]`, `hashes are equal`, `Successful closing`** —
полная успешная передача 121 MB.

**Что это доказывает:**
1. **Прокси-механизм croc несёт bulk-данные корректно** (`comm.NewConnection → dialer.Dial → comm.New`):
   121 MB @ 4.7 MB/s. `socks5://vps` и `wss:`-мост используют **ту же** ветку/глобаль
   (`comm.Socks5Proxy` + `proxy.FromURL`). ⇒ **`wsbridge.go` корректен и для bulk** — причиной
   падения `wss:`→getcroc был только DPI, не код.
2. **Рабочее RU-решение найдено:** `socks5 = личный VPS` (доказано). RU→VPS несёт bulk (VPS вне
   DPI), VPS→142.132.189.179 — тоже. Проще моста: не нужен webrelay, достаточно socks-сервера/`ssh -D`.

**Ключевой нюанс:** `socks5` (VPS) и `wss:` (мост) — **одно поле/глобаль**, взаимоисключающи. Рабочий
конфиг юзера = `socks5=VPS`. `wss:`→публичный getcroc из РФ для bulk непригоден (DPI). Т.о. для юзера с
VPS мост к getcroc **избыточен**; его ниша — «нет VPS, используй публичный getcroc», что из РФ не работает.

## Открытое решение (требует выбора пользователя)

Судьба `wsbridge.go` (+ правки `def()`/`settings.go`), учитывая что `socks5=VPS` решает RU-задачу:
- **(A) Оставить как есть** — код корректен и инертен (включается только при `socks5=wss:`). Полезен в
  сетях, где getcroc НЕ троттлится (не-RU, или RU без правила DPI). Стоимость: ~0 (лежит, не мешает).
- **(B) Расширить `wss:` на свой webrelay**: разрешить `wss://свой-vps/ws` (а не только `wss:`→getcroc) —
  чтобы поднять `croc serve` (webrelay) на VPS и идти через WS. Но `socks5=VPS` строго проще (не нужен
  webrelay), преимущества нет.
- **(C) Убрать `wsbridge.go`** и откатить правки `def()`/`settings.go` — раз `socks5=VPS` решает задачу,
  а публичный getcroc из РФ мёртв для bulk. Упрощение кодовой базы.

Рекомендация: **(A)** — оставить инертный корректный код (поможет не-RU / будущему, если getcroc
разблокируют или поставят за CDN); (C) — если хочется минимализма. (B) без явной потребности не делать.

**Решение пользователя (2026-08-06):** вариант **(A)+частично (B)** — мост оставляем и расширяем:
`wss:` поддерживает не только shorthand до getcroc, но и **явный `wss://<свой-хост>[/ws]`** (свой
webrelay/CDN). Плюс фикс persists `socks5`. См. «Правки 2026-08-06» ниже.

## Правки 2026-08-06: persist `socks5` + кастомный `wss://` URL

Два независимых бага, которые надо починить (малые, хирургические правки).

### Баг 1 — `socks5` (и `connect`) не сохраняется после перезапуска

**Причина:** при старте/открытии настроек `updateRelaySelector()` (вызывается из `settings.go:288`
`relayUpdate()` и `relays.go:314`) применяет текущий/дефолтный профиль через `updateRelayValues`
(`relays.go:147-154`), которая безусловно делает `socks5Binding.Set(relay.Socks5)`. Но профили **не
хранят** Socks5/Connect (`addCurrentRelay` `relays.go:49-50` закомментированы; дефолтный профиль
`getRelays` `relays.go:83-90` их не задаёт) → `relay.Socks5 == ""` → биндинг (и preference) затирается
в `""` на каждом старте.

**Фикс — `relays.go`, `updateRelayValues` (строки 152-153):** не перезаписывать `socks5`/`connect`,
когда профиль их не задаёт (они глобальные, не часть профиля — это и так следует из закомментированных
строк):
```go
// было:
//  socks5Binding.Set(relay.Socks5)
//  connectBinding.Set(relay.Connect)
// стало:
if relay.Socks5 != "" {
    socks5Binding.Set(relay.Socks5)
}
if relay.Connect != "" {
    connectBinding.Set(relay.Connect)
}
```
Тем самым `socks5` сохраняется как глобальная настройка: введённое юзером значение (автосохраняется
через `BindPreferenceString`) больше не затирается применением профиля. Если профиль явно хранит
Socks5 (через ручной «save profile» `addRelay` `relays.go:226`), оно применится — backward-compatible.

**Проверка:** ввести `socks5=socks5://vps:1080` → перезапустить crocson → значение на месте.

### Баг 2 — `wss://<свой-хост>` не работает (заменяется на getcroc)

**Причина:** в `def()` (`send.go:2965-2970`) проверка `strings.HasPrefix(s, "wss:")` ловит и bare
`wss:`, и полный `wss://...`, после чего `s = wsBridgeBase` безусловно перезаписывает URL на
`wss://getcroc.com/ws` → кастомный хост теряется.

**Фикс — `send.go`, блок в `def()` (строки 2965-2970):** различать shorthand и явный URL (`net/url`
уже импортирован в `send.go`):
```go
if strings.HasPrefix(s, "wss:") {
    registerWSBridgeDialer()
    if strings.HasPrefix(s, "wss://") {
        // явный мост: wss://<хост>[/ws] — оставляем как базу, RelayAddress = хост
        if u, err := url.Parse(s); err == nil && u.Host != "" {
            r = u.Hostname()
        } else {
            r = "getcroc.com" // fallback
        }
        // s остаётся как есть (пользовательский URL)
    } else {
        // bare "wss:" — shorthand до публичного getcroc
        s = wsBridgeBase
        r = "getcroc.com"
    }
    r6 = ""
}
```
**Фикс — `wsbridge.go`, `Dial` (после `u := *d.base`):** если в базе нет пути, дефолтить `/ws`
(чтобы `wss://getcroc2.com` без пути тоже работал — конвенция webrelay):
```go
u := *d.base
if u.Path == "" || u.Path == "/" {
    u.Path = "/ws"
}
```

**Поведение после фикса:**
- `wss:` → `wss://getcroc.com/ws`, RelayAddress=getcroc.com (как раньше).
- `wss://getcroc2.com` → `wss://getcroc2.com/ws?port=N`, RelayAddress=getcroc2.com.
- `wss://getcroc2.com/relay` → `wss://getcroc2.com/relay?port=N`.

**Edge-cases (прим.):** URL с userinfo (`wss://u:p@h/ws`) — `Hostname()` корректно вернёт хост, но
наш WS-dialer игнорирует auth (getcroc/webrelay его не требует); `wss://` без хоста → fallback на
getcroc.com. `ws://` (без TLS) намеренно не поддерживается (префикс `wss:`).

### Валидация
- `make arm64 wsl` — компилируется.
- Баг 1: ввести `socks5=socks5://vps` → рестарт → значение сохранено.
- Баг 2a: `socks5=wss:` → (лог) `connected to 'getcroc.com:9009'` через `dialer.Dial`.
- Баг 2b: `socks5=wss://<свой-webrelay>/ws` → `connected to '<свой-хост>:9009'` через `dialer.Dial`.
  (NB: из РФ bulk по-прежнему упрётся в DPI, если хост троттлится — это вне правок; тест валиден в
  сети, где хост доступен для bulk.)

## Правки 2026-08-06 (итерация 2): `/ws` в shorthand + глобальные socks5/connect

**Текущее состояние кода (после правок пользователя):**
- `send.go` `def()`: пользователь упростил — `s = "wss://" + r` (любой путь из URL отбрасывается,
  `r = u.Hostname()`). Т.е. `s` теперь всегда без пути → `/ws` должен добавляться отдельно.
- `wsbridge.go` `Dial`: уже дефолтит `u.Path = "/ws"` при пустом пути (строки 56-60) — т.о. `wss://getcroc2.com`
  уже работает через диалер. Но пользователь просит добавить `/ws` явно.
- `relays.go`: стоит `if relay.Socks5 != "" {…}` guard (итерация 1). Пользователь хочет чище — вообще
  убрать Socks5/Connect из `Relay`, сделав их глобальными.

### Правка A — добавлять `/ws` в `def()` (чтобы `wss://getcroc2.com` работало явно)

**`send.go` `def()` (строка ~2974):** `s = "wss://" + r` → `s = "wss://" + r + "/ws"`.
После этого `s` всегда несёт `/ws` (т.к. `def()` и так отбрасывает введённый путь и берёт только хост).
`wsbridge.go` path-default (56-60) оставить как defense-in-depth (опц. можно убрать — станет мёртвым).

Поведение: `wss://getcroc2.com` → `s="wss://getcroc2.com/ws"` → диалер → `wss://getcroc2.com/ws?port=N`. ✓

**Известное ограничение (вне скоупа):** `r = u.Hostname()` роняет порт webrelay → webrelay предполагается
на :443. `wss://host:8443` скастуется к 443. Если нужен не-443 webrelay — отдельная доработка
(разделять порт webrelay и meeting-порт relay).

### Правка B — убрать `Socks5`/`Connect` из `Relay` → полностью глобальные

Аудит: `.Socks5`/`.Connect` типа `Relay` используются **только в `relays.go`** (struct, закоммент.
`addCurrentRelay`, `updateRelayValues`, `addRelay`); в диплинках не участвуют (`applinks.go:564-565`
закомментированы). `createRelaySelector` вызывается из одного места (`settings.go:139`). Безопасно.

Шаги (все в `relays.go`, кроме вызова в `settings.go`):
1. **`Relay` struct (64-72):** удалить поля `Socks5` (`json:"socks4"`) и `Connect`.
2. **`updateRelayValues` (152-161):** удалить весь if-guard блок (socks5/connect `Set`).
3. **`addRelay` (234-235):** удалить `relay.Socks5, _ = socks5Binding.Get()` и `relay.Connect, _ = connectBinding.Get()`.
4. **`createRelaySelector` сигнатура (134-140):** убрать параметры `socks5Binding, connectBinding binding.String`.
5. **`addCurrentRelay` (49-50):** удалить закомментированные строки (косметика).
6. **`settings.go` (139-146):** убрать аргументы `relaySocks5Binding, relayConnectBinding` из вызова
   `createRelaySelector`. **Сами binding'и (136-137) оставить** — они нужны полям-входам
   (`relaySocks5Entry`/`relayConnectEntry`, 157-158) и напрямую связаны с preferences (`BindPreferenceString`),
   т.е. авто-persist и без профиля.

**Результат:** `socks5`/`connect` — чисто preference-backed глобальные настройки; профиль релея их вообще
не касается → сохраняются между перезапусками без всяких guard'ов. Old saved-профили с Socks5/Connect в
JSON просто игнорируются при unmarshal (поля убраны); глобальная pref `socks5` хранится отдельно и не теряется.

### Валидация (итерация 2)
- `make arm64 wsl` — компилируется (важно: сигнатура `createRelaySelector` изменилась → проверить вызов).
- `socks5=socks5://vps` → рестарт → сохранено; смена профиля релея не сбрасывает socks5.
- `socks5=wss://getcroc2.com` → лог: `dialer.Dial` → `connected to 'getcroc2.com:9009'`, в WS-URL есть `/ws`.


## Out of scope (пока)

- Сама реализация WS→`net.Conn` и правки dial в crocson (следующая сессия).
- Поддержка браузерного клиента crocson.
- Альтернативные обходы (свой relay и т.п.) — отдельная тема.

## Верификация актуальности 2026-08-05

**Сеть из РФ (живая проверка, Ростелеком):** egress `5.139.127.16` (`16.127.139.5.donpac.ru`),
г. Матвеев Курган (Ростовская обл.), AS12389 PJSC Ростелеком, MSK. Premise плана подтверждена вживую:
- **getcroc.com — полностью доступен из РФ**: DNS корректный (`142.132.189.179`, без подмены DPI);
  TCP 443 открыт; TLS валиден (CN=getcroc.com, Let's Encrypt, TLSv1.3); `/healthz` → `ok`.
- **WS-мост работает end-to-end из РФ**: реальный апгрейд `wss://getcroc.com/ws?port=N` (curl `--http1.1`)
  → `HTTP/1.1 101 Switching Protocols` для **9009** (контроль/meeting), **9010** и **9017** (дата).
  Т.к. в `webrelay.go:267-283` dial до backend-relay идёт **до** ответа `101` (иначе `502`), сам `101`
  доказывает одновременно: фронт доступен + WS принят из РФ + co-located `relay` жив. `port=1` → `403`.
- **croc.schollz.com — заблокирован из РФ**: и `:9009`, и `:443` → closed (тот же egress). Прямой relay
  из РФ недоступен — ровно та проблема, которую решает мост.
- Транзитные сбои: ~1 из 5 попыток на 9009 давало обрыв (`000`), на повторе — `101`. Характерно для
  Ростелекома → реализация в crocson должна устойчиво ретраить/переподключаться (croc это умеет).

**Сеть (пробы):**
- `https://getcroc.com/healthz` → `ok` — мост жив.
- `croc.schollz.com:9009` → TCP **closed/unreachable** — предпосылка (прямой relay заблокирован) верна.
- `GET /ws?port=1` → `403 relay port is not allowed`; `port=9009` и `port=9017` доходят до WS-accept
  (allowlist `9009,9010–9017` действует). Доп. кадрирование не требуется.
- `https://getcroc.com/config.js` теперь отдаёт `relayAddress:"relay:9009"` (раньше `croc.schollz.com:9009`) —
  это **косметика для браузера**; crocson ходит через шлюз `/ws` напрямую со своим croc-стеком, значение игнорируется.

**Код (`orig/croc`, HEAD `fea1eca1` — на 75 коммитов выше опорного `1f34c726`):**
- `src/comm/comm.go:23` `MAGIC_BYTES="croc"`, `Write` одним вызовом — без изменений.
- `src/tcp/tcp.go:389` `weakKey={1,2,3}`, PAKE `siec`, `pipe()` сырых байт — без изменений.
- `src/webrelay/webrelay.go`: `websocket.NetConn` (байт-прозрачный), `/ws` route, allowlist, 502 «relay is
  unavailable» — без изменений.

**Дрейф upstream (несущественный для crocson — проверено 2026-08-05):**
- Коммит HEAD `fea1eca1` «feat: use croc.schollz.com temporarily» меняет лишь **compile-time дефолт**
  backend-`RelayHost` (`ipv4.getcroc.com` → `croc.schollz.com`) в `normalizeConfig` (`if RelayHost == ""`).
  **Развёрнутый getcroc.com этот дефолт перебивает флагом `--relay`**, поэтому смена дефолта на работу моста
  **не влияет вообще**. Доказательства:
  - `disco.json` (деплой getcroc.com): web-сервис стартует как `serve --bind 0.0.0.0:8080 --relay relay`,
    где `relay` — **соседний Disco-сервис** (отдельный croc-relay, публикует 9009–9017) в той же деплой-группе.
    Т.е. у getcroc.com **свой co-located relay**, публичный `croc.schollz.com` в цепочке не участвует.
  - `/config.js` отдаёт `relayAddress:"relay:9009"` (а не `croc.schollz.com:9009`/`ipv4.getcroc.com:9009`) —
    прямое подтверждение, что серверный backend = `relay`.
- **Почему блокировка `croc.schollz.com` из РФ роли не играет:** dial к backend-relay идёт **сервер-сайд**
  (web → relay, оба на Disco/Hetzner `142.132.189.179`, вне РФ). Российский crocson стучится только во фронт
  `getcroc.com` (доступен) и не резолвит ни `relay`, ни `croc.schollz.com`.
- Новый upstream `623153d5` «bind PAKE handshakes to participants and sessions» — общая concern по
  версионной совместимости пиров; держать форк `abakCroc/croc` актуальным относительно пира на другой стороне.
- **Реальные риски (не изменились сменой дефолта):** блокировка самого `getcroc.com` из РФ (исходная premise
  плана); либо если оператор сменит `--relay` на глобально-недоступный хост (тогда сломается весь веб-клиент
  getcroc.com, не только crocson).

**Итог:** направление реализации валидно, правок в план не требуется — закрыты только два открытых вопроса
(dial-точка, выбор WS-либы).

## Ссылки

- Коммит: https://github.com/schollz/croc/commit/1f34c726c11c4c886f210ddd5461fd79dc202ff6
- `src/webrelay/webrelay.go` — мост `/ws`
- `src/comm/comm.go` — кадрирование `MAGIC "croc"`
- `src/tcp/tcp.go` — relay: weakKey/PAKE/`pipe`
- `src/cli/cli.go` — команда `croc serve` (флаги `--bind/--relay/--ports`)
- `web/src/protocol/{client,transport,framing}.ts` — эталонный веб-клиент
