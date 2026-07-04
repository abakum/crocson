# Кастомные порты: croc-клиент использует meeting-порт из RelayAddress

## Симптом
Релей поднят на кастомных портах `18909–18913` (пинг `93.178.84.34:18909` → `pong`,
ссылка корректно несёт `18909,18910,…`). Но croc-клиент отправителя ломится на
дефолтный `10.161.115.189:9009` → `actively refused`:
```
croc.go:827: got host '10.161.115.189' and port '9009'
... 10.161.115.189:9009: connectex: ... actively refused
send.go:1581: send: could not connect to 10.161.115.189
```

## Корень
`def()` (`send.go:2905`) возвращает `r` = `relay-address` pref **без порта** и
`ps` = `relay-ports` CSV. croc-библиотека (`abakCroc/croc/src/croc/croc.go:820-828`)
берёт **meeting-порт** из `RelayAddress` через `net.SplitHostPort`; если порта нет —
дефолит до `models.DEFAULT_PORT` (9009):
```go
host, port, _ := net.SplitHostPort(address) // "10.161.115.189" → host='',port=''
if port == "" { host = address; port = models.DEFAULT_PORT } // → 9009
```
`opt.RelayPorts` — это **data-порты** (для параллельной передачи), а не meeting.
Поэтому `RelayAddress` обязан содержать meeting-порт (первый из relay-ports).

Отсутствие порта в `RelayAddress` ломает и **отправителя**, и **получателя** (оба
читают `relay-address` + `relay-ports` и зовут `def()`): получатель тоже шёл бы на
`:9009` вместо `:18909`.

## Решение
В `def()` (`send.go:2905`) добавлять meeting-порт (первый элемент `relay-ports`) к
адресу релея, если тот без порта. Это единая точка: `def()` вызывается и из
`send.go:1362`, и из `recv.go:824` — чинит обе стороны сразу.

Реализация — локальный helper внутри `def()`:
```go
r = defs(relay4, a.Preferences().String("relay-address"))
r6 = defs(relay6, a.Preferences().String("relay6"))
ps = defs(a.Preferences().String("relay-ports"), ports0)
...
// ensurePort: добавляет meeting-порт (первый из relay-ports) к адресу релея,
// если тот без порта; иначе croc дефолит meeting-порт до 9009.
ensurePort := func(addr string) string {
    if addr == "" || strings.HasPrefix(addr, "0") {
        return addr // пусто или прямой IP-режим ("0...") — порт не добавляем
    }
    if _, _, err := net.SplitHostPort(addr); err == nil {
        return addr // уже с портом
    }
    meeting := strings.TrimSpace(strings.SplitN(ps, ",", 2)[0])
    if meeting == "" {
        return addr
    }
    return net.JoinHostPort(addr, meeting)
}
r = ensurePort(r)
r6 = ensurePort(r6)
```

Особенности `ensurePort`:
- `net.SplitHostPort` корректно отличает «без порта» от «с портом» и для IPv4
  (`10.161.115.189` → err → дописать; `10.161.115.189:18909` → ок), и для IPv6
  (`::1` → err → `[::1]:meeting`; `[::1]:9009` → ок).
- `strings.HasPrefix(addr, "0")` — crocson-конвенция прямого IP-режима
  (`recv.go:829` `opt.IP = TrimPrefix(addr,"0")`): для таких адресов meeting-порт
  не нужен, пропуск (иначе ломается ветка прямого IP на получателе).
- meeting-порт берётся из `ps` (defaults → `ports0` = `9009,9010,…`), так что для
  дефолтных портов дописывается `:9009` — поведение не меняется.

## Что НЕ меняется
- `relay-address` **pref** остаётся без порта: `def()` возвращает `r` с портом, но
  не пишет его в pref. Pref заполняется портом-less значениями: из ссылки
  (`send.go:1671` / `recv.go:126` — поле `as`) и из `applyRelayValues`
  (`relay.Address`, без порта). Поэтому формат ссылки не меняется: `relay-address`
  (без порта) + отдельное поле `relay-ports`.
- Линк `toURI`/`setClipboard` (`applinks.go`) — без изменений.
- `opt.RelayPorts` на отправителе — без изменений (уже корректен: split от `ps`).
- Локальный релей (`relayRunCtx`) и логика старта/профиля — без изменений.

## Побочные эффекты / охват
- Чинит отправителя: `RelayAddress = 10.161.115.189:18909` → meeting на 18909.
- Чинит получателя: `RelayAddress = 93.178.84.34:18909` (из ссылки) → meeting на 18909.
- Внешние релеи на дефолтных портах: дописывается `:9009` — нейтрально.
- Прямой IP-режим (`"0..."`) — пропущен, поведение сохранено.

## Импорт
`net`, `strings` уже импортированы в `send.go`.

## Проверка
- `go build ./...`, `go vet ./...` — чисто.
- `make wsl`, `make install`, `make arm64` — собираются.
- Сценарий: поднять локальный релей на host с кастомными `relay-ports`
  (`18909,…`) → отправить → в логе `got host '10.161.115.189' and port '18909'`,
  `trying connection to 10.161.115.189:18909`, соединение установлено.
- Сценарий (получатель): принять по ссылке с публичным IP + `18909,…` →
  `got host '93.178.84.34' and port '18909'`, соединение установлено.
- Сценарий (дефолт): релей на `9009,…` → `port '9009'` (как раньше), регресса нет.
