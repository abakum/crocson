# Fix: локализация не применилась — переводы вносились не в тот файл

## Симптом
После правок подписей чекбоксов и wormhole-строк в UI виден **английский**
текст вместо перевода (для ru-RU и др.).

## Корневая причина (истинная)
gotext использует **два** json-файла на локаль:
- `messages.gotext.json` — **канонический источник** переводов (читается
  gotext при слиянии);
- `out.gotext.json` — **выход** слияния (извлечённые строки + переводы из
  `messages.gotext.json`); перегенерируется каждый прогон.

Переводы были внесены в `out.gotext.json`, а не в `messages.gotext.json`.
Поэтому каждый `go generate` сбрасывал 6 новых строк в `out.gotext.json` и в
`catalog.go` обратно в пусто (старые 54 перевода выживали, т.к. лежат в
`messages.gotext.json`). `en-US` не имеет `messages.gotext.json` — для него
`translation` = `message` (исходный язык).

Подтверждение: `internal/translations/locales/ru-RU/messages.gotext.json`
содержал 57 переведённых id и **0** из 6 новых строк.

## Исправление
1. Добавить 6 переводов в `messages.gotext.json` для `ru-RU, tr-TR, ja-JP,
   zh-CN` (id/message/translation/position).
2. `go generate ./internal/translations/` — gotext смержит переводы из
   `messages.gotext.json` в `out.gotext.json` и перегенерирует `catalog.go`.
3. `go build ./...`.

## Проверка
- `out.gotext.json` каждой локали: 6 строк переведены (не пусто).
- `catalog.go`: в `xx_XXData` появились переведённые строки (индексы более не
  схлопнуты).
- В UI (RU и др.) чекбоксы Local и wormhole-сообщения локализованы.
- `go build ./...` без ошибок.

## Правило на будущее
**Любой новый/изменённый перевод вносится в `messages.gotext.json`**, затем
`go generate`. Правка только `out.gotext.json`/`catalog.go` будет потеряна при
следующей регенерации.
