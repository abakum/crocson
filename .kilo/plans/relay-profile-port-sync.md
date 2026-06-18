# Синхронизация профиля релея и локального посредника

## Цель
Устранить рассинхрон между настройками профиля/портов и реально запущенным
локальным релеем (когда в код/ссылку уходят одни порты, а релей слушает другие).
Сделать профиль источником истины, а выбор хоста — применяющим профиль.

## Корневая причина (контекст)
- `hostSelect.OnChanged` (settings.go:196) находит профиль через
  `getRelayByAddress` (по `relay.Address`) и **читает** `relay.Ports` в локальную
  переменную для `relayRunCtx`, но не гарантирует, что bindings (а значит entry и
  `toURI`/ссылка) получат те же значения.
- Смена профиля в `relaySelect` (relays.go:173) и редактирование полей пишут
  bindings, но **не перезапускают** уже работающий релей → он держит старые порты.
- Релей стартует только при выборе хоста; правки профиля игнорируются до ручного
  OFF/ON хоста.

## Решение (вариант пользователя)

### Б. Выбор хоста: применить профиль целиком (если есть) и читать ports/pass из формы
Подбор профиля оставляем **по адресу посредника**, как и раньше —
`getRelayByAddress(a, next)`. Эта функция сравнивает поле **`relay.Address`**
(адрес посредника, без порта) с `next`; имя профиля `Name` в поиске не участвует.
(Матч по имени `Name==host` отменён — он не нужен.)
При смене хоста (`next != OFF`, ветка старта):
1. Найти профиль через `getRelayByAddress(a, next)` — совпадение по
   **`relay.Address` (без порта) == next**. Матч срабатывает, только если у
   какого-то профиля адрес посредника равен выбранному хосту; иначе профиль не
   найден.
2. Если найден (`relay.Name != ""`) — применить его ко **всем полям формы**
   (address, address6, ports, password, socks5, connect), переиспользовав
   существующую логику `updateRelayValues` из `createRelaySelector` (её надо
   выставить наружу — см. 2b).
   Если не найден — bindings не меняем (в форме текущие значения).
3. **В любом случае** взять значения из формы:
   `ports, _ := relayPortsBinding.Get()`; `pass, _ := relayPasswordBinding.Get()`.
   Единый путь чтения — нет ветвления «из профиля / из формы».
4. Bind-адрес релея = `next` (выбранный интерфейс). При заданном `host`
   `relay-address` в `setClipboard` всё равно перекрывается `host`.
5. `pass`/`ports` для `relayRunCtx`; пустые → `DEFAULT_PASSPHRASE`/`ports0`
   (фолбэк также есть внутри `relayRunCtx`).
6. `setRelayName`/`relaySelect.SetSelected`/`relayUpdate` **не зовём** (применяем
   поля напрямую, не через OnChanged селектора профилей) → хук
   `onRelayProfileApplied` не срабатывает, цикла нет.

### А. Перезапуск ТОЛЬКО при смене/сохранении профиля И изменении портов/пароля
Триггеры — исключительно действия с профилем (не ручная правка полей):
1. Выбор профиля в `relaySelect` (`relays.go:173` OnChanged).
2. Сохранение/обновление профиля кнопкой/Enter (`addRelay`, `relays.go:188`).

**Условие рестарта:** профиль сменился, И при этом `ports` или `password` реально
отличаются от тех, на которых релей сейчас работает. Если сменился только профиль,
а `ports` и `password` те же — релей **не** перезапускается.

Мост между relays.go и логикой старта в settings.go — через глобальный хук по
образцу уже существующего `relayUpdateUI`:
- В relays.go: `var onRelayProfileApplied func()`; вызов `if onRelayProfileApplied
  != nil { onRelayProfileApplied() }` в конце `relaySelect.OnChanged` и в конце
  `addRelay` (после `updateRelaySelector()`). **Не** вызывать из `updateRelaySelector`
  (чтобы инициализация/refresh не давали спурных рестартов).
- В settings.go: `onRelayProfileApplied = restartRelayIfRunning`.

`restartRelayIfRunning()` (замыкание в settings.go, выполняется на UI-потоке):
- `host, _ := hostBinding.Get()`; если `host == "" || host == OFF` → return.
- **В. Проверка адреса:** `addr, _ := relayAddressBinding.Get()`. Если `addr != ""`
  и `cleanAddress(addr) != host` — активный профиль больше не соответствует
  забинженному хосту (это профиль другого/внешнего релея) →
  `hostSelect.SetSelected(OFF)` (каскадно стопает релей + тост + сброс host) и
  return (см. раздел В).
- `ports, _ := relayPortsBinding.Get()`; `pass, _ := relayPasswordBinding.Get()`.
- Если `ports == runningPorts && pass == runningPass` → return (ничего существенного
  не изменилось).
- фолбэки: `ports=="" → ports0`; `pass=="" → DEFAULT_PASSPHRASE`.
- `ctc()`; `relayGeneration++`; `prev = OFF`; `startRelay(pass, host, ports)`.

`runningPorts` и `runningPass` выставляются в `startRelay` при каждом старте.

Дебаунс НЕ нужен (событие профиля — одиночное, не посимвольный ввод).
Флаг `applyingProfile` НЕ нужен (Б пишет bindings напрямую в settings.go и не
зовёт хуки relays.go, поэтому спурного триггера нет).

### В. Остановка локального посредника при рассогласовании адреса профиля и host
Локальный релей забинжен на `host`. Если при ручной смене/сохранении профиля
(`relaySelect.OnChanged`, `addRelay`) адрес посредника выбранного профиля
`relay.Address` перестал совпадать с `host` — значит активный профиль теперь
описывает **другой** (внешний или иной хост) релей, и локальный более неактуален →
останавливаем его.
- Проверка живёт в начале `restartRelayIfRunning` (выше), до проверки ports/pass.
- Сравнение через `cleanAddress(addr)` (без порта) == `host` (`host` всегда bare IP).
- `addr == ""` НЕ считаем рассогласованием (пустой адрес не блокируем) → релей не
  трогаем, переходим к обычной логике ports/pass.
- Остановка делается через `hostSelect.SetSelected(OFF)` (а не прямой `stopRelay()`):
  это каскадно зовёт `OnChanged(OFF)` → `stopRelay()` (тост «Релей остановлен»),
  сбрасывает `prev = OFF` и выставляет `host` pref в OFF. Один тост, без двойного
  вызова. Плюс: профиль теперь указывает на внешний релей → host=OFF автоматически
  включает режим внешнего релея в `setClipboard`.
- См. допущение 6 (альтернатива — не сбрасывать host).

### Тосты при старте и остановке релея
- `startRelay(...)`: после успешного запуска горутины —
  `NewToast(w, lp("Relay started")+": "+host+":"+ports).Show()`.
- `stopRelay()` (новый helper в settings.go): `ctc()`; `relayGeneration++`;
  `prev = OFF`; `NewToast(w, lp("Relay stopped")).Show()`.
- Локализация (`lp` = `langPrinter.Sprintf`, gotext): добавить строки `Relay started`
  и `Relay stopped` (id == message, как уже сделано для `Relay`) в
  `internal/translations/locales/<lang>/messages.gotext.json` для каждого языка
  (ru-RU и т.д.). Без записей gotext отдаёт сам id (английский fallback) — работать
  будет, но без перевода.
- `stopRelay()` используется:
  1. ветка `next == OFF` в `hostSelect.OnChanged` (ручное выключение; сюда же
     каскадно попадает остановка по рассогласованию адреса и естественное
     завершение горутины через `SetSelected(OFF)`).
- При **рестарте** (`restartRelayIfRunning`) отдельного тоста остановки НЕ показываем
  (`ctc()` без тоста + `startRelay` с тостом старта) — пользователь видит один тост
  «запущен».

## Защита от гонки (обязательно)
Релей крутится в горутине; по завершении он зовёт
`hostSelect.SetSelected(OFF)`. При перезапуске старая горутина ещё жива и могла бы
сбросить только что стартовавший новый релей.
- Ввести счётчик поколений `relayGeneration`; каждая стартующая горутина
  запоминает `gen`.
- В колбэке завершения: `if gen != relayGeneration { return }`.
- Любой перезапуск делает `relayGeneration++`, `ctc()`, `prev = OFF`, затем
  `startRelay(...)`.

## Реализация (settings.go + relays.go)

### 1. settings.go — рефакторинг старта/остановки релея в замыкания
Вынести тело горутины из `hostSelect.OnChanged` в
`startRelay(pass, host, ports string)`, которое:
- `relayGeneration++`, `gen := relayGeneration`
- `ctx, ctc = context.WithCancel(appCtx)`
- `prev = host`; `runningPorts = ports`; `runningPass = pass`
- `disableLocalBinding.Set(true); disableLocalCheck.Refresh()`
- `NewToast(w, lp("Relay started")+": "+host+":"+ports).Show()`
- `go relayRunCtx(...)` с проверкой `gen != relayGeneration` перед
  `hostSelect.SetSelected(OFF)` (лог — как сейчас).

`stopRelay()`:
- `ctc()`; `relayGeneration++`; `prev = OFF`
- `NewToast(w, lp("Relay stopped")).Show()`

### 2. settings.go — переписать `hostSelect.OnChanged` (ветки старта и стопа)
Ветка старта (`next != OFF`, `prev == OFF`):
- `relay := getRelayByAddress(a, next)`.
- `if relay.Name != "" { applyRelayValues(relay) }` (применяем все поля профиля;
  `applyRelayValues` — выставленный из `createRelaySelector` замыкающий
  `updateRelayValues`).
- `ports, _ := relayPortsBinding.Get()`; `pass, _ := relayPasswordBinding.Get()`
  (всегда из формы, единый путь).
- `startRelay(pass, next, ports)` (bind = `next`).
Ветка стопа (`next == OFF`): заменить инлайн `ctc()` на вызов `stopRelay()`
(чтобы показать тост остановки).
Ветка `prev != OFF` (рекурсия через OFF) — оставить как есть.

### 2c. relays.go — вынести `cleanAddress` в пакетную функцию
- Сейчас `cleanAddress` — локальное замыкание внутри `getRelayByAddress`
  (`relays.go:306`). Вынести в пакетную `func cleanAddress(addr string) string`
  (та же логика: отрезать `:port`, если после `:` число).
- Использовать её и в `getRelayByAddress`, и в `restartRelayIfRunning` (раздел В).

### 2b. relays.go — выставить `updateRelayValues` наружу
- `createRelaySelector` сейчас возвращает `(relayControls, updateRelaySelector)`.
  Добавить третье возвращаемое значение `applyRelay func(relay Relay)`, указывающее
  на существующий `updateRelayValues`. Сам `updateRelayValues` не менять (он уже
  проставляет address/address6/ports/password/socks5/connect).
- В settings.go вызов станет
  `relayControls, relayUpdate, applyRelayValues := createRelaySelector(...)`.

### 3. settings.go — `restartRelayIfRunning` + регистрация хука
- Определить замыкание `restartRelayIfRunning` (см. А).
- После `hostSelect.SetSelected(s)` (инициализация): `onRelayProfileApplied =
  restartRelayIfRunning`.

### 4. relays.go — глобальный хук
- `var onRelayProfileApplied func()` (рядом с `relayUpdateUI`).
- Вызов в конце `relaySelect.OnChanged` и в конце `addRelay` (с nil-проверкой).

### 5. Импорт
`"time"` в settings.go НЕ нужен (дебаунса нет). Иных изменений импортов нет.

## Новые переменные замыкания (в settings-функции)
- `relayGeneration int`
- `runningPorts string` — порты текущего релея
- `runningPass string` — пароль текущего релея

## Глобальные хуки (relays.go)
- `onRelayProfileApplied func()` — выставляется в settings.go в
  `restartRelayIfRunning`; вызывается из `relaySelect.OnChanged` и `addRelay`.

## Границы / что НЕ меняется
- `hostSelect` по-прежнему показывает локальные IP + `0.0.0.0` (вариант
  «hostSelect = имена профилей» отвергнут).
- Подбор профиля в `hostSelect` остаётся **по адресу** через `getRelayByAddress`
  (матч по имени `Name==host` отвергнут — он не нужен).
- Двухкликовое переключение хоста (OFF → повторный выбор) сохранено.
- `getRelayByAddress` в relays.go используется как и раньше из `hostSelect`.
- `relayRunCtx` (croc.go:218), `toURI`/`setClipboard` (applinks.go) — без изменений.
- Режим внешнего релея (`host == OFF`, `relay-address` из настроек) не затрагивается.

## Допущения (ПОДТВЕРЖДЕНЫ пользователем)
1. **Bind-адрес = `next`** (выбранный интерфейс). ✅
2. **Перезапуск только по событиям профиля** (`relaySelect.OnChanged`, `addRelay`),
   и **только если** ports ИЛИ password изменились; ручная правка полей релей НЕ
   перезапускает. ✅
3. **Подбор профиля по адресу посредника** (`getRelayByAddress(next)` — сравнивает
   `relay.Address` с `next`, имя профиля не участвует). При найденном — применяем
   **все поля** формы (address/address6/ports/password/socks5/connect) через
   `updateRelayValues`; ports/pass для старта релея читаем из формы.
   `setRelayName`/`relaySelect.SetSelected`/`relayUpdate` не зовём. ✅
4. **Остановка при `relay.Address != host`**: при ручной смене/сохранении профиля,
   если адрес посредника перестал совпадать с забинженным `host` (и адрес не пустой)
   — локальный релей останавливается. `addr == ""` остановку не вызывает. ✅
5. **Тосты**: `lp("Relay started")+": "+host+":"+ports` при старте;
   `lp("Relay stopped")` при остановке (ручное OFF, рассогласование адреса,
   естественное завершение). При рестарте — только тост старта. Новые строки
   локализации добавить в `messages.gotext.json`. ✅
6. **При остановке по рассогласованию адреса (раздел В) `host` сбрасывается в OFF** ✅
   (`hostSelect.SetSelected(OFF)`): каскадно стопает релей, тост «Релей остановлен»,
   UI честно показывает OFF, автоматически включается режим внешнего релея. После
   возврата к подходящему профилю хост выбирается заново.

## Проверка
- `go build ./...` и `go vet ./...` — чисто.
- Сборка под все целевые платформы (правки в settings.go/relays.go затрагивают общий
  код — проверить компиляцию везде):
  - `make wsl`     — Windows (GOOS=windows, mingw).
  - `make install` — Linux (`go install`).
  - `make arm64`   — Android release (`fyne package -os android/arm64 --release --sign`).
- Сценарий 1: запустить релей на хосте → сменить профиль в `relaySelect` на профиль
  с другими портами/паролем → в логе `starting croc relay ...@host:[новые порты]`
  без ручного OFF/ON.
- Сценарий 1b: сменить профиль на профиль с **теми же** ports и password → релей
  **не** перезапускается (работает дальше).
- Сценарий 2 (ручная правка НЕ перезапускает): запустить релей → отредактировать
  поле ports/password → релей остаётся на старых параметрах (как сейчас); рестарт
  только если сохранить/перевыбрать профиль с изменившимися ports/password.
- Сценарий 3: выбрать хост, для которого есть профиль с совпадающим `Address`
  (адресный матч) → в поля формы подставляются **все** поля этого профиля, релей
  стартует на его ports/pass (bind = выбранный хост); выбрать хост без такого
  профиля → релей стартует на текущих значениях полей формы.
- Сценарий 4: быстрое многократное переключение хоста/профиля → новый релей
  стартует один раз, старая горутина не сбрасывает селектор (генерации).
- Сценарий 5 (остановка по адресу): запустить локальный релей на host
  `10.161.115.189` → выбрать в `relaySelect` профиль с `Address` ≠ `10.161.115.189`
  (напр. внешний релей) → релей останавливается, тост «Релей остановлен»,
  `hostSelect` сбрасывается в OFF.
- Сценарий 5b: то же, но выбрать профиль с пустым `Address` → релей **не**
  останавливается (пустой адрес игнорируется).
- Сценарий 6 (тосты): старт релея → тост «Релей запущен: …»; выключение хоста в OFF
  → тост «Релей остановлен»; рестарт по смене ports/pass → один тост «запущен».
- Внешний пинг `nc -vz <publicIP> <порт>` после смены профиля отвечает на новый
  диапазон портов.
