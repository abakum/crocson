# Санитизация невидимых символов в именах файлов (ZWSP и др.)

## Контекст / проблема

При передаче файла `8 июн._ 17.07​.m4a` (голосовая заметка iOS, пересланная через
Telegram/мессенджер) приёмник падает с ошибкой:

```
non-graphical unicode: e2808b U+8203 in '...'
receive: non-graphical unicode: ...
```

ZWSP (U+8203, Zero-Width Space) и подобные невидимые руны (ZWNJ U+200C, ZWJ U+200D,
BOM U+FEFF, контрольные символы) оказываются в имени файла. crocson **не создаёт** их —
он лишь проксирует имя, полученное от Android `ContentResolver.DISPLAY_NAME` через
`uriBase` (`for_android.go:701` / `for_android0.go:21`), дословно вплоть до валидации
в форке croc. Валидация `utils.ValidFileName` режектит такие имена (Issue croc #595).

## Ключевое архитектурное ограничение

В форке croc поле `FileInfo.Name` **двойного назначения**:
- передаётся получателю;
- используется отправителем для чтения исходного файла с диска:
  `croc.go:520` и `croc.go:2153` строят путь как `FolderSource/Name`.

Поэтому санитизировать `Name` внутри форка на стороне **отправителя** нельзя —
сломается чтение исходного файла. Отправная санитизация должна идти в **crocson на
стадии копирования** в `fyne/send/`: чистое имя оказывается на диске, и дальше
`croc.GetFilesInfo` → `stat.Name()` отдаёт чистое имя само.

## Подход

**Вырезать** невидимые руны (а не whitelist конкретных). Критерий удаления руны `r`:
`!unicode.IsGraphic(r) || !unicode.IsPrint(r)`. Остальные (вкл. кириллицу, CJK, эмодзи,
пробелы, `.`, `-`, `_`) оставить. `ValidFileName` не ослаблять — он остаётся защитой от
`..`, абсолютных путей, path-separator в basename.

Решение принято пользователем: санитизация **на приёме и на передаче**.

---

## Задачи

### A. Приём — форк croc (`../abakCroc/croc`)

1. **`src/utils/utils.go`** — добавить функцию:
   ```go
   // SanitizeFileName removes invisible/non-graphic/non-print runes from fname.
   func SanitizeFileName(fname string) string {
       var b strings.Builder
       for _, r := range fname {
           if unicode.IsGraphic(r) && unicode.IsPrint(r) {
               b.WriteRune(r)
           }
       }
       return b.String()
   }
   ```
   `ValidFileName` (стр. 793) **не менять**.

2. **`src/croc/croc.go`, `processMessageFileInfo` (стр. ~1402–1427)** — перед
   `utils.ValidFileName` (стр. 1423) прогнать имя и папку через санитизатор и
   записать обратно:
   ```go
   c.FilesToTransfer[i].Name         = utils.SanitizeFileName(fi.Name)
   c.FilesToTransfer[i].FolderRemote = utils.SanitizeFileName(filepath.Clean(fi.FolderRemote))
   ```
   (санитизация FolderRemote — после `filepath.Clean`, как сейчас).

3. **`src/utils/utils_test.go`** — обновить `TestValidFileName`:
   - существующий кейс с ZWSP (`"D中文.cslouglas​"`) остаётся проверкой, что
     `ValidFileName` по-прежнему режектит грязное имя;
   - добавить `TestSanitizeFileName`: `SanitizeFileName("D中文.cslouglas​") == "D中文.cslouglas"`;
   - добавить ассерт `assert.Nil(t, ValidFileName(SanitizeFileName("...с ZWSP...")))`.

### B. Передача — crocson (стадия копирования в `fyne/send/`)

1. **Общий хелпер** `sanitizeFileName(s string) string` (рядом с `replace`,
   `for_android.go:725`), вырезающий невидимые руны тем же критерием
   (`!unicode.IsGraphic(r) || !unicode.IsPrint(r)`). Импорт `unicode`.

2. **`for_android.go:701` `uriBase`** — применить `sanitizeName` к результату
   `getFileName`/`base` (т.е. прокинуть внутрь `base`/`replace` или поверх).
3. **`for_android0.go:21` `uriBase`** — `return sanitizeName(uri.Name())`.

   Эффект: при кэшировании/симлинке в `fyne/send/` файл получает чистое имя на диске
   → `croc.GetFilesInfo` (вызов в `send.go:1293`) читает чистый `stat.Name()` →
   передаётся чистое имя → приёмник (и форк, и upstream croc) проходит `ValidFileName`.

### C. (опционально) crocson `zip.go`

Применить ту же санитизацию к именам zip-записей в копии `ValidFileName`
(`zip.go:425`) при распаковке — тот же класс невидимых символов. Можно отложить, если
zip-сценарий не актуален.

---

## Проверка

1. **Воспроизведение бага**: повторить сценарий из логов — голосовая заметка iOS
   `.m4a` с ZWSP передаётся Android (crocson) → Windows (crocson).
   Ожидаемый результат: успешный приём, сохранённое имя
   `8 июн._ 17.07.m4a` (без ZWSP), без ошибки `non-graphical unicode`.
2. **Тесты форка**: `go test ./src/utils/` в `../abakCroc/croc`.
3. **Сборка/lint crocson**: `go vet ./...` и штатная сборка (Android + desktop).
4. **Регресс имён**: убедиться, что кириллица/CJK/эмодзи/пробелы в именах
   сохраняются (не вырезаются).

## Затронутые файлы

- `../abakCroc/croc/src/utils/utils.go` (+ `SanitizeFileName`)
- `../abakCroc/croc/src/utils/utils_test.go`
- `../abakCroc/croc/src/croc/croc.go` (`processMessageFileInfo`)
- `crocson/for_android.go` (`uriBase`, `base`, `replace`/новый `sanitizeName`)
- `crocson/for_android0.go` (`uriBase`)
- (опц.) `crocson/zip.go`

## Риски / заметки

- Санитизация имени на диске при стадинге меняет видимое пользователю имя кэшированного
  файла в `fyne/send/` — это желательно (имя становится читаемым).
- Подход вырезания (а не whitelist) надёжнее: покрывает любые будущие невидимые руны,
  не требуя поддержки списка.
- Приём в форке остаётся защитой даже от upstream-отправителей, которых мы не
  контролируем (главная гарантия интеропа).
