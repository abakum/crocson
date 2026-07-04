# Двойной старт релея при включении host: сравнение raw vs fallback

## Симптом
- Старт приложения с **включенным** `host` → релей стартует **один раз**, без ошибок бинда. ✓
- Старт с **выключенным** `host` и последующим включением → `starting croc relay`
  выводится **дважды** одновременно → `bind: Only one usage of each socket address
  …` (EADDRINUSE) на 18910/18911.

В логе дважды:
```
croc.go:229: starting croc relay pass123@10.161.115.189:[18909 18910 18911 18912 18913]
croc.go:229: starting croc relay pass123@10.161.115.189:[18909 18910 18911 18912 18913]
… error listening on 10.161.115.189:18910: bind: …
```

## Корень
`startRelay` вызывается дважды. Второй вызов — из хука `restartRelayIfRunning`
(`settings.go:296`), который должен был no-op, но ошибочно ушёл в рестарт из-за
**несимметричного сравнения**.

`runningPorts`/`runningPass` выставляются в `startRelay` значениями, к которым
вызывающая сторона **уже применила fallback** (пусто → `ports0`/`DEFAULT_PASSPHRASE`).
А в `restartRelayIfRunning` сравнение идёт с **raw**-значениями из bindings ДО
применения fallback (`settings.go:308-318`):
```go
ports, _ := relayPortsBinding.Get()
pass, _ := relayPasswordBinding.Get()
if ports == runningPorts && pass == runningPass {  // raw vs уже-fallback!
    return
}
if strings.TrimSpace(ports) == "" { ports = ports0 }       // fallback ПОСЛЕ сравнения
if strings.TrimSpace(pass) == "" { pass = DEFAULT_PASSPHRASE }
```
У профиля пароль пустой → `startRelay` сохранил `runningPass = "pass123"` (fallback),
а хук сравнивает raw `pass = ""` с `"pass123"` → **неравенство** → рестарт →
второй `startRelay` → конфликт бинда. (Порты `18909,…` непустые, поэтому по port
сходилось, а по password — нет; в логе пароль и есть `pass123`.)

## Почему кейс 1 работает, а кейс 2 — нет
Хук `onRelayProfileApplied` срабатывает от `relaySelect.OnChanged`, который
запускается `relayUpdate()` в конце ветки старта `hostSelect` (синхронизация Name).
- **Кейс 1 (host ON на старте)**: `createRelaySelector` при инициализации уже
  выровнял `relaySelect` на текущий профиль → `relayUpdate()` →
  `relaySelect.SetSelected(name)` не меняет выбор → `OnChanged` **не** зовётся →
  хук не работает → один старт.
- **Кейс 2 (включение host позже)**: `relaySelect` стоит на другом профиле →
  `SetSelected` меняет выбор → `OnChanged` → хук → `restartRelayIfRunning` →
  баг-сравнение → рестарт → второй старт.

## Решение
В `restartRelayIfRunning` (`settings.go:296`) применить fallback **до** сравнения,
чтобы обе стороны были в одинаковой (нормализованной) форме:
```go
ports, _ := relayPortsBinding.Get()
pass, _ := relayPasswordBinding.Get()
if strings.TrimSpace(ports) == "" {
    ports = ports0
}
if strings.TrimSpace(pass) == "" {
    pass = DEFAULT_PASSPHRASE
}
if ports == runningPorts && pass == runningPass {
    return
}
// дальнейший рестарт (relayGeneration++/ctc()/prev=OFF/startRelay) — без изменений
```
Т.е. переставить два `if TrimSpace==""` **выше** блока сравнения.

## Эффект
- Пустой пароль/порты в профиле: raw `""` → нормализуется в `pass123`/`ports0` →
  совпадает с `runningPass`/`runningPorts` → no-op, без рестарта.
- Кейс 2: один `starting croc relay`, без EADDRINUSE.
- Та же правка чинит и корректный кейс смены профиля: переход между профилями с
  пустым паролем больше не даёт ложного рестарта.

## Что НЕ меняется
- Логика `startRelay`/`stopRelay`, поколений (`relayGeneration`), хука
  `onRelayProfileApplied`, адресной остановки (`cleanAddress(addr) != host`).
- `def()`/`ensurePort`, `relays.go`.

## Проверка
- `go build ./...`, `go vet ./...` — чисто; `make wsl`/`make install`/`make arm64`.
- Кейс 2: запустить с `host` OFF → включить хост → в логе **одна** строка
  `starting croc relay …`, ошибок `bind:` нет.
- Кейс 1: запуск с `host` ON → одна строка (регресса нет).
- Смена профиля с пустым паролем на профиль с пустым паролем → без рестарта.
