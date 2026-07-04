# План: согласие с политикой конфиденциальности (правило 7.5 AppGallery)

Цель: реализовать в crocson запрос согласия с политикой конфиденциальности
при первом запуске и доступ к ней из интерфейса. Снимет ограничение показа
в выбранных странах из отчёта проверки.

URL политики: **https://abakum.github.io/croc/privacy-policy.html** (HTTP 200,
страница существует).

---

## A1. Текст политики — ГОТОВО (размещено)
- Страница размещена: https://abakum.github.io/croc/privacy-policy.html
- Проверить содержание: приложение передаёт файлы P2P (релеи croc),
  **не собирает/не хранит персональные данные**; разрешения
  (`INTERNET`, доступ к файлам, multicast) и их назначение.
- При желании добавить локализованные версии (ru/en/zh-CN).

---

## A2. Диалог согласия при первом запуске
Реализовать в `main.go` **до любой сетевой активности и до `w.ShowAndRun()`**.

Логика:
- Проверять флаг `a.Preferences().Bool("privacy-accepted")`.
- Если false — показывать диалог (Fyne `dialog.NewCustom`).
  - Текст — **без i18n**, показывается сразу на **5 языках** приложения
    (en, tr, ja, zh, ru), чтобы было понятно при любом языке системы.
    Чтобы строки помещались по ширине, фразы делаем КОРОТКИМИ.
    Контент диалога = VBox из:
      1) Короткий призыв «прочитайте» на 5 языках (по одной короткой строке):
         • English:   "Please read"
         • Türkçe:    "Lütfen okuyun"
         • 日本語:     "お読みください"
         • 中文:       "请阅读"
         • Русский:   "Пожалуйста прочитайте"
      2) Пять ОТДЕЛЬНЫХ гиперссылок `widget.NewHyperlink` (по одной на язык),
         каждая с текстом «Политика конфиденциальности» на своём языке и
         одинаковым URL = PrivacyPolicyURL:
         • "Privacy Policy"
         • "Gizlilik Politikası"
         • "プライバシーポリシー"
         • "隐私政策"
         • "Политика конфиденциальности"
    (Отдельные ссылки гарантированно переносятся по строкам.)
  - **Кнопки БЕЗ текста, только иконки** (понятно без перевода):
      • Accept  → `widget.NewButtonWithIcon("", theme.ConfirmIcon(), ...)`
        (зелёная галочка ✓) → `SetBool("privacy-accepted", true)` →
        продолжить запуск.
      • Decline → `widget.NewButtonWithIcon("", theme.CancelIcon(), ...)`
        (крестик ✗) → завершить приложение (`cleanup`/`os.Exit`).
    Кнопки разместить в HBox снизу контента диалога.
  - Диалог сделать **незакрываемым мимо кнопок** (без крестика/Dismissible),
    чтобы пользователь обязан выбрать Accept или Decline. В Fyne — построить
    кастомный `dialog.NewCustom` без кнопки закрытия (убрать стандартный
    dismiss) или использовать `dialog.NewCustomWithoutButtons` + свои кнопки.
- Согласие хранится в preferences, повторно не показывается.

i18n НЕ затрагивается: новые строки в `lp()` и перегенерацию `catalog.go`
не добавляем.

Место вставки: проверка флага — в `main()` до `w.ShowAndRun()`. Но сам
диалог нельзя показать до запуска цикла событий (`ShowAndRun()` блокирует
до закрытия окна). Поэтому показ диалога выполнять ПОСЛЕ старта цикла:
- Либо через `a.Lifecycle().SetOnStartedForeground(func(){ ... проверить
  флаг и показать диалог ... })`;
- Либо запустить горутину перед `w.ShowAndRun()`, которая через
  `fyne.Do(...)` показывает диалог (с учётом, что флаг ещё не установлен —
  UI при этом можно скрыть/заблокировать до выбора).

i18n (`gotext`/`catalog.go`) не трогаем — текст фиксированный.

---

## A3. Чекбокс согласия в About (вместо ссылки)
- На вкладке About (`about.go`, `aboutTabItem`) вместо гиперссылки добавить
  `widget.Check` с подписью «Privacy Policy».
- Начальное состояние: `check.SetChecked(a.Preferences().Bool(privacyAcceptedKey))`
  (галочка = согласие принято).
- `OnChanged(checked bool)`:
  - `checked == true` → просто `SetBool(privacyAcceptedKey, true)` (без диалога).
  - `checked == false` (пользователь СНЯЛ галочку) → открыть диалог согласия:
    * Accept  → повторно подтвердить: вернуть галочку `true` (с временным
      отключением OnChanged, чтобы не зациклиться) + `SetBool true`.
    * Decline → `revokeConsent(a, w)`: `cleanup(w)` →
      `os.Remove(filepath.Join(tempDir, "preferences.json"))` → `os.Exit(0)`.
      (Стирание файла настроек + выход; при следующем запуске снова покажется
      стартовый диалог.)
- Иконки кнопок диалога прежние: ✓ Accept / ✗ Decline, без текста.

## Доработка 2 (после теста на десктопе)

### D4. Accept не должен закрывать приложение (баг D1)
Симптом: после D1 нажатие ✓ (Accept) тоже закрывает приложение.

Причина: в обработчике Accept порядок `d.Hide()` → `respond(true)`. Fyne
синхронно вызывает `SetOnClosed` во время `Hide()`, поэтому первым срабатывает
`respond(false)` из OnClosed и поглощает `sync.Once`; последующий
`respond(true)` — no-op → ведёт себя как Decline.

Решение в `showPrivacyConsent` (`privacy.go`):
- Accept: сначала `respond(true)`, ПОТОМ `d.Hide()`:
  ```
  a.Preferences().SetBool(privacyAcceptedKey, true)
  respond(true)
  d.Hide()
  ```
- Decline: для единообразия тоже `respond(false)` до `d.Hide()`:
  ```
  respond(false)
  d.Hide()
  ```
- `SetOnClosed(func(){ respond(false) })` остаётся (dismissal мимо кнопок);
  once-гард гарантирует, что он не сработает после кнопки.

### D5. Центрировать пары; русский — допустимо в 2 строки
Сейчас пары `HBox(метка, гиперссылка)` выровнены по левому краю в VBox.
Длинный русский вариант («Пожалуйста прочитайте» + «Политика
конфиденциальности») не помещается по ширине и переносится — это допустимо.
Остальные (en, tr, ja, zh) должны быть в одну строку и **по центру**.

Решение: каждую пару обернуть в центрирующий контейнер:
```
container.New(layout.NewCenterLayout(),
    container.NewHBox(widget.NewLabel("<фраза>"),
        widget.NewHyperlink("<название>", policyURL("<код>"))))
```
VBox из 5 таких (центрированных) строк. Импортировать
`fyne.io/fyne/v2/layout` в privacy.go. Текст/коды/URL — как в D2.

### D6. Чекбокс About: любая смена состояния открывает диалог
Ранее (A3) диалог открывался только при СНЯТИИ галочки; постановка просто
сохраняла согласие. Новое требование: **любое** переключение чекбокса
(и установка, и снятие) открывает диалог согласия.

Поведение `OnChanged` в `about.go` (`onPrivacyChanged`):
- Всегда вызывать `showPrivacyConsent(a, w, onResult)` (игнорируя направление).
- onResult(true) (Accept):
  - `SetBool(privacyAcceptedKey, true)`;
  - выставить галочку `true` с временным отключением `OnChanged`
    (чтобы не зациклиться), затем вернуть `OnChanged = onPrivacyChanged`.
- onResult(false) (Decline / закрытие мимо кнопок):
  - `revokeConsent(w)` (cleanup → удалить preferences.json → os.Exit).
- Начальное состояние чекбокса — по флагу `privacyAcceptedKey` (как раньше).
- D6 заменяет логику A3 «только при снятии».
- Совместимо с D4 (respond до Hide) — диалог корректно вернёт результат.

### D1. Диалог: доставлять результат кнопкой на всех платформах
Проблема: на десктопе тап ✗ (Decline) не закрывает приложение и не стирает
`preferences.json`, т.к. результат `onResult(false)` доставлялся только через
`SetOnClosed`, а на десктопе колбэк закрытия по кнопке не срабатывает
надёжно. Accept работает, потому что вызывает `onResult(true)` напрямую.

Решение в `showPrivacyConsent` (`privacy.go`):
- Ввести `sync.Once` + обёртку `respond(accepted bool)`, вызывающую
  `onResult` ровно один раз.
- Accept → `SetBool true` + `d.Hide()` + `respond(true)`.
- Decline → `d.Hide()` + `respond(false)` (напрямую, не ждать OnClosed).
- `SetOnClosed` → `respond(false)` (под страх — dismissal мимо кнопок;
  once-гард делает его no-op, если уже ответили).
- `revokeConsent` оставить как есть: путь
  `filepath.Join(tempDir, "preferences.json")` корректен и на десктопе, и на
  Android (tempDir = `a.Storage().RootURI().Path()` = Fyne storageRoot).
- Импортировать `sync` в privacy.go.

### D2. Диалог: НЕ разрывать текст и гиперссылку + параметр языка
Сейчас 5 отдельных `Label` + 5 отдельных гиперссылок — фраза и ссылка
разнесены. Объединить в каждой паре: фраза «прочитайте» + кликабельное
название политики рядом (в одной строке). Каждая ссылка несёт параметр
языка `?lang=<код>`, чтобы страница показала нужный язык (см. D3).

Решение (простое, без RichText): по одной паре на язык =
`container.NewHBox(widget.NewLabel("<фраза>"),
widget.NewHyperlink("<название политики>", policyURL("<код>")))`.
VBox из 5 таких HBox.
- Хелпер `policyURL(code string) *url.URL`:
  `u, _ := url.Parse(PrivacyPolicyURL); q := u.Query(); q.Set("lang", code);
   u.RawQuery = q.Encode(); return u` (через `net/url`).
- Пары (фраза → название политики → код):
  • "Please read"          → "Privacy Policy"                  → en-US
  • "Lütfen okuyun"        → "Gizlilik Politikası"             → tr-TR
  • "お読みください"          → "プライバシーポリシー"               → ja-JP
  • "请阅读"                 → "隐私政策"                          → zh-CN
  • "Пожалуйста прочитайте" → "Политика конфиденциальности"    → ru-RU
- Импортировать `net/url` в privacy.go.

### D3. Страница политики: читать `?lang=` из URL (один язык)
Файл `/home/koka/src/abakum.github.io/croc/privacy-policy.html`.
Сейчас язык выбирается чекбоксами и хранится в localStorage (`crocson.privacy.langs`),
URL-параметра нет. Добавить в IIFE (после `checkboxes`, ДО `loadStates`):
- Прочитать `var lang = new URLSearchParams(location.search).get('lang');`
- Если `lang` задан и есть среди значений чекбоксов (en-US/tr-TR/ja-JP/zh-CN/ru-RU):
  - выставить состояние «только этот язык» (он = true, остальные = false);
  - проставить `cb.checked` соответственно;
  - `saveStates(states)`; `applyVisibility()`;
  - НЕ читать localStorage в этом случае (параметр приоритетнее).
- Если параметра нет — текущая логика (loadStates → localStorage → DEFAULTS).
- Дополнительно: при наличии параметра можно обновить `history.replaceState`,
  чтобы убрать `?lang=` из адресной строки (опционально, не обязательно).

> Примечание: базовый рефакторинг `showPrivacyConsent(a,w,onResult)` и
> `revokeConsent` уже выполнен ранее. D1/D2 — это исправления поверх него.

---

## Не входит в этот план (отложено)
- B (сбой запуска / диагностика) — отдельно.
- A4 (URL политики в AppGallery Connect) и C (публикация) — отдельно.

## Доработка 3 (после теста)

### D7. Русский — фраза над ссылкой (2 строки), остальные в одну строку
Симптом: `centerPair` = `CenterLayout(HBox(label,link))` не переносит — HBox
не Wrap, диалог расширяется, и русский остаётся в одну строку.

Решение в `privacy.go`: параметризовать пары вертикальностью.
- Сделать `pairRow(label, link fyne.CanvasObject, vertical bool) fyne.CanvasObject`:
  - vertical==true (русский): 
    `container.NewVBox(centered(label), centered(link))`, где
    `centered(o) = container.New(layout.NewCenterLayout(), o)` — фраза по центру
    на 1-й строке, гиперссылка по центру на 2-й.
  - vertical==false (остальные):
    `container.New(layout.NewCenterLayout(), container.NewHBox(label, link))`.
- Вызовы: en/tr/ja/zh → `pairRow(..., false)`; ru → `pairRow(..., true)`.
- Удалить/заменить старый `centerPair`.

### D8. Центрировать кнопки Accept/Decline
Симптом: `container.NewBorder(nil, buttons, ...)` ставит HBox кнопок слева.
Решение: `buttons := container.NewHBox(layout.NewSpacer(), accept, decline,
layout.NewSpacer())` — две кнопки по центру строки. Border bottom остаётся.

### D9. Синхронизировать чекбокс About с флагом согласия
Симптом: чекбокс в About строится при создании окна (флаг ещё false);
согласие первого запуска ставит флаг ПОЗЖЕ (через OnStarted) → чекбокс остаётся
снимаемым/неотмеченным (stale).

Решение:
- В `about.go` сделать чекбокс и его синхронизатор пакетными переменными:
  - `var privacyCheck *widget.Check`
  - `var privacyCheckSync func()` — замыкание: выставляет
    `privacyCheck.SetChecked(flag)` с временным отключением `OnChanged`,
    затем возвращает `OnChanged = onPrivacyChanged`.
- В `aboutTabItem`: присвоить `privacyCheck`, задать `privacyCheckSync`,
  вызвать `privacyCheckSync()` для начального состояния.
- В `OnSelectedTab[ABOUTi]` (about.go) вызвать `privacyCheckSync()` в начале —
  обновлять при каждом открытии вкладки.
- В `showPrivacyConsentOnStart` (privacy.go) в ветке accept (onResult true)
  вызвать `if privacyCheckSync != nil { privacyCheckSync() }` — обновить чекбокс
  сразу после согласия первого запуска.
- Сам About-флоу (onResult(true)) уже делает SetChecked(true) — оставить;
  privacyCheckSync там избыточен, но не вредит.

### Проверка
- `go build ./...` и `go vet .` без ошибок.
- Десктоп: первый запуск → Accept → открыть About → чекбокс отмечен.
- Диалог: ru в 2 строки (фраза над ссылкой), остальные в 1 строку по центру;
## Доработка 4 (после теста)

### D10. Чекбокс-предложение в About + заголовок попапа (фикс. текст, БЕЗ lp)
Локализацию пока НЕ делаем («Пока не локализуй») — используем фиксированный
английский текст (заглушки под будущие `lp()`).

a) Заголовок попапа согласия (`privacy.go`, `showPrivacyConsent`):
- `dialog.NewCustomWithoutButtons("Accept", ...)` вместо `"Privacy Policy"`.

b) Строка чекбокса в About (`about.go`, `tightVBoxLayout`):
- `privacyCheck = widget.NewCheck("", nil)` (пустая метка — только квадратик).
- Собрать строку-предложение:
  `privacyRow := container.NewHBox(
      widget.NewLabel("Accept"), privacyCheck, widget.NewLabel("of Privacy Policy"))`
- В `tightVBoxLayout` заменить `privacyCheck` на `privacyRow`.
- `privacyCheckSync` / `onPrivacyChanged` не трогать (работают с `privacyCheck`).
- Целевой вид (англ.): «Accept [x] of Privacy Policy»;
  в будущем с `lp()`: ru «Принимаю [x] Политику конфиденциальности».

c) (Отложено) Локализация строк «Accept» и «of Privacy Policy» через `lp()` +
   переводы в `internal/translations/locales/*/messages.gotext.json` +
   перегенерация `catalog.go` (gotext). Не выполнять сейчас.

### Проверка
- `go build` + `go vet` без ошибок.
- About: строка «Accept [x] of Privacy Policy», квадратик чекбокса по центру фразы.
- Попап согласия: заголовок «Accept», тело — 5 пар (ru в 2 строки), кнопки ✓/✗ по центру.

## Порядок выполнения (Доработка 4)
1. D10a — заголовок попапа «Accept» (privacy.go).
2. D10b — строка «Accept [x] of Privacy Policy» в About (about.go).
3. `go build` + `go vet`.
(Доработки 1–3 — D1–D9 — уже реализованы.)
1. D7 — параметризовать `pairRow`; ru вертикально (2 строки), остальные горизонтально.
2. D8 — центрировать кнопки ✓/✗ через spacers.
3. D9 — пакетные `privacyCheck`/`privacyCheckSync`; синхронизация в about init,
   OnSelectedTab[ABOUTi] и после consent первого запуска.
4. `go build` + `go vet`.
(Доработка 2 — D4/D5/D6 — уже реализована.)
