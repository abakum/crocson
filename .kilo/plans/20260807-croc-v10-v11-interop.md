# Dual v10+v11 интероп для crocson: port `codephrase`+`pakekey` в форк

> Статус: **implementation-ready**. Все дизайн-решения сняты.
> Апстрим `/home/koka/src/orig/croc` (HEAD `fea1eca1`). Форк `abakCroc/croc`
> (`@v10.0.0-20260704080127-24111f72b04b`, локально `/home/koka/src/abakCroc/croc`).
> **Граница разрыва:** upstream-коммиты `284c9f08` (codephrase) и `623153d5` (pakekey),
> оба 2026-07-31; форк и любые сборки до неё — доразрывные (v10).

## Контекст (верифицировано по коду)

- Разрыв **двусторонний**: old→new падает на peer-PAKE (новый fail-closed `incompatiblePakeVersionError`,
  `Version=0≠2`, `croc.go:2212`); new→old — на формате кода (old режет `secret[:4]`/`secret[5:]`,
  four-word room не совпадает). Половинчатый backport (только `codephrase`) **бессмысленен**.
- Оба пакета **самодостаточны и стабильны с ввода**: `codephrase` (stdlib), `pakekey`
  (`pake/v3`+`hkdf`, уже в графе). Соль комнаты `"croc"` идентична old/upstream.
- **PAKE инициирует получатель** (`croc.go:1906`); `senderInfo`/`ReconnectVersion` — после PAKE
  (Step2) ⇒ до PAKE версию пира узнать нельзя.
- Форк: 6 точек сплита `SharedSecret[:4]/[5:]` в `croc.go` (228,267,665,854,1125,1571);
  своя фича `transferOverLocalRelay` (`8c17635`, pake `pake1`/`pake2`).
- Старый PAKE слабее (нет identity/transcript binding, нет key confirmation) — hardening класса
  активного MITM, **не** раскрытие данных (SPAKE2 держит offline-перебор; PAKE-passphrase защищает
  данные; relay секретов не знает).

## Снятые решения

1. **Цель:** **dual v10+v11** (интероп с обоими; v11-only теряет большинство текущих пиров, v10-only
   изолирует от getcroc-веб/свежего CLI).
2. **Детекция версии пира — два сигнала, retry НЕ делаем:**
   - **Получатель** (initiator): по **формату введённого кода** (`fourLowercaseWords` → v11, иначе → v10).
     Реализовано как `c.usePakekey = (codeComponents.Format == codephrase.FormatFourWord)`.
     ⚠ **Расхождение с upstream:** в upstream `Components.Format` **не читается** в Go-пути
     (`croc.go:262-263` берёт только `.RoomName`/`.PAKEPassphrase`; `Format` живёт лишь в тесте
     `utils_test.go:310` и wasm-мосте `web/wasm/main.go:407`). Upstream-получатель **безусловно**
     шлёт `Version: pakekey.ProtocolVersion` (`croc.go:1906`) — там v11-only, диспетчеризация не нужна.
     В форке она **обязательна** для dual (иначе PIN-код от v10 не примется). Это НЕ мёртвый код.
   - **Отправитель** (responder): по полю **`Version`** входящего PAKE (2 → v11 `pakekey`, иначе → v10 `pake.InitCurve`).
   - **Без retry.** В форке получатель может занимать комнату **первым** (`roomInfo.first` + кастомная
     авторизация `f9bf52a`/`b487280` в `tcp.go`, **в отличие от апстрима**) → повторный PAKE после
     провала на уже занятой комнате некорректен. Эвристика по формату надёжна для **автогенерируемых**
     кодов (все реалистичные пиры); редкий edge (ручной `--code` с mismatch формата) просто падает —
     юзер перезапускает вручную.
3. **Генерация кода:** **PIN по умолчанию** (полный send-dual: v10+v11 парсят room). Room-сила для
   crocson **неважна** — relay тупая труба без секретов, данные защищает PAKE-passphrase; реальный
   relay crocson = личный VPS (коллизий ~0, публичной DoS-поверхности нет). **four-word — опц. чекбокс
   «v11»** в секции Transfer Options (`settings.go`) → `codephrase.Generate()` (сильная комната) для
   power-user'ов на публичных relay. НЕ дефолт.
   **Чекбокс ТОЛЬКО выбирает формат генерируемого секрета** (`send.go:308`) — он **НЕ управляет
   совместимостью**. Dual-stack (порт `codephrase`+`pakekey` + dual PAKE-dispatch) **всегда активен**,
   независимо от чекбокса: crocson всегда **принимает** и от v10, и от v11, и всегда отвечает пиру по
   его версии (формат кода / поле `Version`). Единственное следствие — свойство самого формата four-word,
   не чекбокса: v10-получатель не парсит four-word room ⇒ при **вкл** SEND доходит только до v11/crocson;
   при **выкл** (PIN, дефолт) — до обеих версий.
4. **Локальный relay** (`transferOverLocalRelay`): **оставить на v10-PAKE** (same-fork LAN работает;
   кросс-версия на LAN fallback'ает на relay — не смертельно).
5. **Срок:** v10-нога несётся пока v10-пиры актуальны; sunset когда v11 станет нормой.

> Эвристика детекта по секрету надёжна для **автогенерируемых** кодов (v10 всегда PIN-цифры, v11
> всегда four-word из wordlist), но **не** для ручных `--code`, оказавшихся 4 строчными словами
> (`fourLowercaseWords` чисто структурный, без wordlist/версии) — латентный edge даже upstream.
> Retry для него **не предусмотрен** (решение 2): такой код падает, юзер перезапускает вручную.

## Задачи реализации (в форке `abakCroc/croc`)

1. **Скопировать пакеты verbatim** с `orig/croc` HEAD: `src/codephrase/` (включая `wordlists/*`) и
   `src/pakekey/`.
2. **`src/message/message.go`:** добавить поле `Version int` (`json:"v,omitempty"`) и `TypePAKEConfirm`
   (поле `Bytes2` уже есть в форке). Сверить с `623153d5` diff message.go.
3. **`src/croc/croc.go` — room/passphrase:** заменить прямые `SharedSecret[:4]/[5:]` на один
   `codephrase.Parse(c.Options.SharedSecret)` → `c.Options.RoomName = comp.RoomName` + поле
   `c.pakePassphrase = comp.PAKEPassphrase`. Для PIN/legacy даёт **идентичный** результат (legacy-ветка
   `Parse` = `secret[:4]+\"croc\"`, соль та же) ⇒ обратно-совместимо. Покрыть все 6 точек.
4. **`src/croc/croc.go` — dual PAKE (прививка pakekey РЯДОМ с существующим v10, без wholesale-импорта
   upstream-переписи — она конфликтует с кастомом форка):**
   - **Responder (отправитель)** в `processMessagePake`: по `m.Version` — `2` → ветка `pakekey`
     (Init/Derive/Confirm как в upstream `croc.go:2220-2330`); иначе → существующая v10-ветка
     `pake.InitCurve`.
   - **Initiator (получатель)** при старте PAKE (`croc.go:1906`-аналог): `fourLowercaseWords(secret)`
     → true: `pakekey.Init` + `Version: ProtocolVersion` в первом `TypePAKE`; false: существующий
     v10 `pake.InitCurve`, без `Version`.
5. **Генерация кода — PIN дефолт + чекбокс «v11»:**
   - **Форк `utils.GetRandomName`:** оставить PIN (без правок) — дефолт генерации.
   - **crocson `settings.go:484-495`** (секция Transfer Options, форма `transferForm`): добавить
     form-item после «no-compress» (строка 488), по конвенции соседних чекбоксов:
     ```go
     widget.NewFormItem("v11", widget.NewCheckWithData(lp("Use four-word code (v11)"), binding.BindPreferenceBool("v11-code", a.Preferences()))),
     ```
     preference-ключ `"v11-code"` (bool, по умолчанию false). Текст FormItem — `"v11"`.
   - **crocson `send.go:308`** (`entry.SetText(utils.GetRandomName())`): обернуть в проверку
     `if a.Preferences().Bool("v11-code")` → `code, _ := codephrase.Generate(); entry.SetText(code)`
     (импорт `github.com/schollz/croc/v10/src/codephrase` из форка), иначе прежний `utils.GetRandomName()` (PIN).
6. **Локальный relay:** `transferOverLocalRelay` НЕ трогать (v10-PAKE).
7. **crocson `go.mod`:** bump `replace github.com/schollz/croc/v10 => ...abakCroc/croc/v10` на новый
   pseudo-version/тег форка после коммита правок. Детект версии пира и PAKE-dispatch живут **в форке**;
   **единственные crocson-правки** — UI-чекбокс «v11» + dispatch генерации кода (`settings.go:488` +
   `send.go:308`, задача 5).
8. **Диагностическое логирование формата (debug-лог):** в `src/croc/croc.go` форка добавить `log.Debug`
   на точках dual-диспетчеризации — пишется в crocson `debuglog.txt` через общий `schollz/logger`
   (`main.go:288-293` `SetOutput(MultiWriter(crocdebuglog,…))`). Только диагностика, без UI/изменения
   поведения:
   - **Получатель** (init, ~`croc.go:1279`): `"recipient code format: four-word (v11)"` /
     `"legacy (v10)"` по `c.usePakekey`/`codeComponents.Format`.
   - **Отправитель** (responder, в `processMessagePake` после детекта `m.Version`):
     `"peer PAKE version: v11"` (Version==2) / `"v10"` (иначе).

## Риски / edge-cases

- **Прививка pakekey в кастомный croc.go:** форк +174 строк в croc.go (локальный relay, авторизация) —
  grafting требует ручной аккуратности, cherry-pick `623153d5` целиком **не** встанет чисто.
- **Эвристика детекта:** ручной four-word `--code` от v10-пира → misdetection → передача падает, юзер
  перезапускает вручную (retry убран, см. решение 2).
- **Расхождение форка:** несём обе PAKE-ветки = постоянный tax до sunset v10. Каждое будущее upstream-
  изменение PAKE/протокола — реинтеграция против dual-форка.
- **Безопасность v10-ноги:** для crocson↔crocson на PIN (v10-PAKE) — слабее (нет key confirmation/binding);
  осознанный tradeoff (PAKE-passphrase защищает данные; MITM-класс на приватном relay неактуален).

## Валидация

- `make arm64 wsl` — сборка обоих таргетов crocson (тянет обновлённый форк).
- **Матрица интеропа** (4 угла): crocson↔getcroc-веб, crocson↔свежий `croc` CLI (v11),
  crocson↔старый `croc` CLI (v10), crocson↔crocson.
- Sender: PIN-код доходит до v10 **и** v11 получателя (dispatch по `Version`).
- Receiver: four-word-код от v11 → v11-PAKE; PIN-код от v10 → v10-PAKE (детект по формату).
- **Чекбокс «v11»** (settings.go): выкл → код PIN (доходит обеим версиям); вкл → four-word
  (`codephrase.Generate()`), доходит только v11/crocson-получателям. Сохраняется между запусками
  (`BindPreferenceBool("v11-code")`).
- room-совпадение проверить хешами (как в разведке: legacy `secret[:4]+\"croc\"` идентичен old/new).
- Edge: ручной `--code` с mismatch формата (напр. v10-отправитель с four-word-кодом) → передача
  падает (**retry не предусмотрен**, см. решение 2); автогенерируемые коды immune.

## Ссылки

- `orig/croc`: `284c9f08` (codephrase), `623153d5` (pakekey), `efd8aec` (база форка).
- `src/codephrase/codephrase.go:96-116` (Parse, дискриминатор `fourLowercaseWords`),
  `src/pakekey/pakekey.go`, `src/croc/croc.go:1906,2212,2532`.
- Форк `abakCroc/croc` @ `24111f72b04b`: `src/utils/utils.go:285,300` (PIN), `src/croc/croc.go:228,267,665,854,1125,1571`
  (точки сплита), `8c17635` (`transferOverLocalRelay`).
- Отдельный план: `.kilo/plans/20260724-getcroc-ws-relay-bridge.md` (транспорт; не пересекается с код-форматом).
