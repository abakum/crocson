# Не ставить принудительно no-local при старте релея; взводить DisableLocal при операции

## Симптом/пожелание
При включении локального посредника (`host`) принудительно выставляется pref
`disable-local = true` (`startRelay` → `disableLocalBinding.Set(true)`), что ещё и
переключает чекбокс «Disable local relay when sending» в UI. Хочется: pref/чекбокс
не трогать, а эффективный признак `DisableLocal` применять в момент send (и recv).

## Почему DisableLocal=true при host всё-таки нужен
croc при `DisableLocal=false` поднимает **свой** встроенный local-relay
(`abakCroc/croc/src/croc/croc.go:800-809`, `setupLocalRelay()`) на тех же
`RelayPorts`. Если уже крутится наш релей (`relayRunCtx` на 18909–18913), второй
попытается забиндить те же порты → `EADDRINUSE` ( ровно тот конфликт, что лечили
раньше). Поэтому при включённом `host` croc-клиенту **обязательно** нужно
`DisableLocal=true` — но через `opt`, а не через pref/чекбокс.

## Решение

### 1. settings.go — убрать принудительную запись pref в `startRelay`
Удалить из `startRelay` (сейчас строки 213-214):
```go
disableLocalBinding.Set(true)
disableLocalCheck.Refresh()
```
Pref `disable-local` и чекбокс больше не модифицируются при старте релея —
отражают выбор пользователя.

### 2. send.go — взводить `opt.DisableLocal` при send, если host включён
После формирования `opt` (рядом с `opt.OnlyLocal = ...`, send.go:1365):
```go
if host := a.Preferences().String("host"); host != "" && host != OFF {
    opt.DisableLocal = true // свой релей уже слушает RelayPorts — не давать croc второй
}
```
Эффективно: `opt.DisableLocal = pref || hostEnabled`. В struct-literal
`DisableLocal: a.Preferences().Bool("disable-local")` (send.go:1343) остаётся как
база; host-on принудительно форсирует true.

### 3. recv.go — НЕ трогаем
На стороне получателя `DisableLocal` читается, но **не вызывает** `setupLocalRelay`:
в `Receive()` (`abakCroc/croc/src/croc/croc.go:1006`) `if !DisableLocal && !isIPset`
включает лишь **peer-discovery** (multicast-поиск отправителя в LAN). Бинд
`RelayPorts`/`setupLocalRelay` — только у отправителя (`croc.go:800-809`). Значит
конфликта бинда на recv нет, и форсировать `DisableLocal=true` там не нужно — более
того, это отключило бы LAN-discovery получателя. Поэтому `recv.go:812`
`DisableLocal: a.Preferences().Bool("disable-local")` оставляем как есть
(чекбокс пользователя управляет discovery).

**Подтверждение из CLI croc** (`abakCroc/croc/src/cli/cli.go`):
- `--no-local` (`DisableLocal`) — флаг **только команды send** (`cli.go:75`,
  внутри `Flags` команды `send`): `croc send --no-local` есть, `croc --no-local` нет.
- `--local` (`OnlyLocal`) — **глобальный** флаг (`cli.go:124`): `croc --local send`
  и `croc --local` (receive).
Т.е. по самому дизайну croc `DisableLocal` — прерогатива отправителя. Это сходится с
нашим выводом: форсировать только в `send.go`.

### (опц.) helper
Проверка `host != "" && host != OFF` повторяется (applinks.go:519, send, recv).
Можно вынести `func hostEnabled(a fyne.App) bool { h := a.Preferences().String("host"); return h != "" && h != OFF }`
— по желанию, для устранения дублирования.

## Что НЕ меняется
- `recv.go`: `opt.DisableLocal` остаётся из pref (управляет только LAN-discovery).
- `opt.OnlyLocal`, адресная логика, `def()`/`ensurePort`, старт/стоп релея.
- Поведение при host OFF: `opt.DisableLocal = pref` (как раньше).
- saveConfig (settings.go:649) сохраняет реальный `opt.DisableLocal` только в режиме
  remember — теперь это значение уже не «исказится» принудительным pref.

## Проверка
- `go build ./...`, `go vet ./...`; `make wsl`/`make install`/`make arm64`.
- Включить host → чекбокс «Disable local relay when sending» **не** отмечается
  принудительно (остаётся в своём состоянии).
- Отправить при включённом host → в логе **один** релей, без `setupLocalRelay`/
  `EADDRINUSE`; croc не поднимает второй local-relay.
- Отправить при выключенном host с отмеченным чекбоксом → `DisableLocal=true` (по
  pref); с снятым → croc использует свой local-relay (как раньше).

## Открытый вопрос
Решён: применять **только в send.go** (на recv `DisableLocal` не вызывает
`setupLocalRelay` и не даёт конфликта бинда — форсировать не нужно).
