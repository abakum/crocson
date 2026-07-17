# recv: единый просмотрщик текста с копированием по согласию

## Контекст

На странице `recv.go` для текстовых/hash-файлов (`clipString != ""`, см. `validHash` —
`.txt` с hex-именем) кнопка «копировать» (`saveButton`, `recv.go:316`) сейчас
немедленно копирует содержимое в буфер, а затем открывает диалог сохранения файла
(`dialogFileSave(..., textDialog=true)`). При отказе от сохранения дополнительно
открывается окно просмотра (`qr.showTextDialog()`, `recv.go:521`).

Цель: единый просмотрщик, открываемый **сразу по клику** на кнопку копирования, без
диалога сохранения. Копирование в буфер — **только при нажатии кнопки копирования**
(согласие); отмена/закрытие/жест назад — **без копирования**. Один и тот же диалог
используется и в QR-секции, и в recv (синхронизация поведения). Заголовок «Clipboard»
убирается (пустой заголовок).

## Решения

- Один унифицированный метод `qr.showTextDialog(text string)`; вызывается из QR-секции
  (с `qr.currentText`) и из recv (с `clipString`).
- Диалог без заголовка (`dialog.NewCustomWithoutButtons("")`), без стандартных кнопок.
- Две иконки-кнопки в нижнем HBox (как в `privacy.go`):
  - `copyBtn` = `theme.ContentCopyIcon()` → `qr.app.Clipboard().SetContent(text)` + `d.Hide()`.
  - `cancelBtn` = `theme.CancelIcon()` (только иконка, без локализации текста) → `d.Hide()`.
- Жест назад / закрытие без кнопки копирования = нет копирования (`SetOnClosed` не копирует,
  только восстанавливает размер окна).
- В `recv.go` для `clipString != ""`: **не** копировать сразу и **не** открывать диалог
  сохранения — открыть просмотрщик и `return`.
- Чистка мёртвого кода: параметр `textDialog` и ветка `qr.showTextDialog()` после отмены
  сохранения больше не нужны (просмотрщик открывается напрямую из кнопки).

## Изменения

### 1. `applinks.go` — рефакторинг `showTextDialog` (стр. 1198–1238)

Сменить сигнатуру: `func (qr *QR) showTextDialog(text string)`, использовать `text`
вместо `qr.currentText`. Тело:

- `var d dialog.Dialog` (заявить до кнопок — они ссылаются на `d.Hide()`).
- Сохранить `textEntry` (`NewMultiLineEntry`, `SetText(text)`, `TextWrapWord`) и
  `wrapButton` (переключатель wrap on/off, как сейчас).
- `copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
  qr.app.Clipboard().SetContent(text); d.Hide() })`.
- `cancelBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() { d.Hide() })`.
- `content := container.NewBorder(wrapButton, container.NewHBox(copyBtn, cancelBtn),
  nil, nil, container.NewVScroll(textEntry))`.
- `d = dialog.NewCustomWithoutButtons("", content, qr.window)` (пустой заголовок).
- Сохранить resize-логику для десктопа (`if !(isAndroid || asMobile || qr.window.FullScreen())`)
  и `SetOnClosed` — только восстановление размера окна, без копирования.
- `d.Resize(qr.window.Canvas().Size())`; `d.Show()`.
- Новых импортов не требуется (`dialog`, `container`, `theme`, `widget` уже есть).

### 2. `applinks.go:331` — обновить вызова по сигнатуре

`qr.link.OnTapped = qr.showTextDialog` →
`qr.link.OnTapped = func() { qr.showTextDialog(qr.currentText) }`.

### 3. `recv.go` — обработчик `saveButton` (стр. 316–355)

Для текстовых файлов открыть просмотрщик и выйти, без мгновенной вставки и без диалога
сохранения:

```go
saveButton := widget.NewButtonWithIcon("", iconSB, func() {
    if clipString != "" {
        qr.showTextDialog(clipString)
        return
    }
    if isMobile || asMobile {
        if isLinkDir(dst) {
            // ... существующий блок зипования каталога ...
            return
        }
    }
    dialogFileSave(dst, w, false)
})
```

### 4. `recv.go` — чистка мёртвого `textDialog`

- `recv.go:58` объявление: `dialogFileSave func(src string, parent fyne.Window)`
  (убрать `textDialog bool`).
- `recv.go:478` определение: `dialogFileSave = func(src string, parent fyne.Window)`.
- `recv.go:347` вызов: `dialogFileSave(pathZip, w)` (убрать `, false`).
- `recv.go:354` вызов: `dialogFileSave(dst, w)` (после правки п.3 здесь всегда non-text;
  убрать `, false`).
- `recv.go:519–525`: удалить `if textDialog { qr.showTextDialog() }` (стр. 521–523),
  оставив:
  ```go
  } else if destination == nil {
      log.Debug("folder selection canceled")
      return
  }
  ```

## Уточнения (follow-up)

### A. Копировать актуальный (отредактированный) текст

Баг: кнопка копирования копирует `text` (значение на момент создания диалога), поэтому
правки в `textEntry` теряются. Копировать надо текущее содержимое поля.

В `showTextDialog` (`applinks.go`, обработчик `copyBtn`) заменить:

```go
qr.app.Clipboard().SetContent(text)
```

на:

```go
qr.app.Clipboard().SetContent(textEntry.Text)
```

`textEntry` объявлен до `copyBtn` и захватывается по ссылке, поэтому `textEntry.Text`
всегда отражает актуальный (возможно отредактированный) текст.

### B. Центрировать нижние кнопки

Нижний ряд кнопок сейчас прижат влево (`container.NewHBox(copyBtn, cancelBtn)`). Отцентровать
его через `layout.NewCenterLayout()` (как `centered` в `privacy.go:61-63`; `layout` уже
импортирован в `applinks.go:19`). В `container.NewBorder` заменить нижний виджет:

```go
container.New(layout.NewCenterLayout(), container.NewHBox(copyBtn, cancelBtn)),
```

## Риски / поведение

- QR-ссылка была просмотром (одна кнопка OK) → станет «копировать по согласию» (copy +
  cancel). Это и есть требуемая синхронизация поведения.
- «Убрать заголовок» трактуется как пустой текст заголовка; сама полоса заголовка
  остаётся (полное удаление требует кастомного виджета — out of scope).
- `qr.window == w` (т.к. `qr = NewQR(a, w)` в `settings.go:499`), resize-логика
  корректна для диалога из recv.
- Жест назад / закрытие = нет копирования (как «отмена»).

## Проверка

- `make arm64 wsl` (AGENTS.md) — сборка Android + Windows, покрывает компиляцию
  изменённых `.go`.
- Вручную на recv: текст/hash-файл в корзине → нажать копировать → открывается
  просмотрщик **без** диалога сохранения и без заголовка; иконка копирования → содержимое
  в буфере; иконка отмены → закрытие без копирования; закрытие/назад → без копирования.
- В QR-секции: клик по ссылке → тот же просмотрщик с `currentText`, копирование по
  согласию.
