# SOP: добавление нового перевода (форк gotext `abakum/text`)

Повторяемый рецепт добавления перевода для новой строки `lp("...")`, чтобы не ломать рабочий `catalog.go`. Выполнять строго по шагам и сверяться с реальным поведением форка на каждом шаге.

## Контекст / факты
- Переводы живут в `internal/translations/`:
  - `catalog.go` — генерируется (`package translations`, заголовок `DO NOT EDIT`). **Не править руками.**
  - `locales/<lang>/messages.gotext.json` — каноническое хранилище переводов (суперсет, ~64 ключа); **из него печётся каталог**.
  - `locales/<lang>/out.gotext.json` — свежая экстракция из исходников (~61 ключ); нужна, чтобы **увидеть** новый ключ.
- Инструмент: форк `github.com/abakum/text/cmd/gotext` (директива `//go:generate` в `internal/translations/translations.go`).
- Языки: `en-US` (исходник), `tr-TR`, `ja-JP`, `zh-CN`, `ru-RU`.
- gotext медленный: компилит cgo/glfw, ~4–6 мин, ~2.5 ГБ ОЗУ. Запускать **в фоне** / таймаут ≥600с.
- **Запуск только** через `go generate ./internal/translations/` (go generate заходит в пакет translations → корректный `package translations`). Прямой запуск `gotext` из корня репо даёт `package main` → сборка падает (`found packages main (catalog.go) and translations (translations.go)`).
- «Missing entry for …» в выводе gotext — шум, не блокируют.

## Рецепт (по шагам, с проверкой после каждого)

### Шаг 0 — исходник
1. Добавить вызов `lp("New string")` в Go-коде.
2. `go build ./...` — собирается; до обновления каталога `lp` фолбэчит на английский исходник.

### Шаг 1 — экстракция (долго)
1. Запустить в фоне: `go generate ./internal/translations/` (таймаут ≥600с).
2. Ожидаемый результат:
   - В каждом `locales/<lang>/out.gotext.json` появился новый ключ с пустым `translation`.
   - `catalog.go` перегенерирован (новый ключ присутствует, но без перевода → фолбэк en).
3. Контроль: `grep '"New string"' internal/translations/locales/ru-RU/out.gotext.json` — ключ есть. Если нет — вызов был неверный (проверить, что запускали через `go generate`, а не gotext из корня).

### Шаг 2 — перевести пустые поля в `out.gotext.json`
1. В каждом `locales/<lang>/out.gotext.json` найти записи с пустым `"translation": ""` (это и есть новые ключи) и вписать правильный перевод. Существующие (непустые) переводы не трогать — gotext уже влил их в out из messages на шаге 1.
2. `en-US` не трогать (исходник).
3. Контроль: `grep -B2 '"translation": ""' out.gotext.json` — пустых переводов не осталось.

### Шаг 3 — сохранить out в `messages.gotext.json` (копирование целиком)
1. Скопировать `out.gotext.json` → `messages.gotext.json` для каждого языка:
   `cp locales/<lang>/out.gotext.json locales/<lang>/messages.gotext.json`
2. Это **перезапись**, а не merge: `messages.gotext.json` становится точной копией актуальной экстракции (устаревшие ключи, которых уже нет в исходниках, prune'ятся). На проверенном примере: было 65 ключей → стало 62.
3. Поскольку out уже содержит все переводы (влитые из messages + новые), каталог формально не меняется — но это правило безусловное.

### Шаг 3b — запекание (обязательно, всегда после шага 3)
1. **Всегда** после копирования out→messages повторить `go generate ./internal/translations/` в фоне (таймаут ≥600с), чтобы вшить переводы в `catalog.go`. Без этого шага каталог не актуален.

### Шаг 4 — проверка
1. `grep '<перевод>' internal/translations/catalog.go` для каждого языка — переводы вшиты.
2. `sed -n '4p' internal/translations/catalog.go` → ровно `package translations` (не `main`).
3. `go build ./...` и `go vet ./...` — чисто.
4. **Кросс-платформенная сборка**: `make wsl install amd64` — должна пройти без ошибок (windows-сборка + `go install` + `fyne package -os android/amd64 --release --sign`). Это та самая цель, что падала с `found packages main (catalog.go) and translations ...` при `package main`; успешный прогон подтверждает, что каталог валиден для android.
5. Запустить приложение, переключить язык, убедиться, что новая строка отображается переведённой.

## Готчи / риски
- Запуск gotext из корня репо → `package main` в catalog.go → `make amd64` / `fyne package` падает. Только `go generate ./internal/translations/`.
- Каждый прогон ~4–6 мин; не обрывать по дефолтному 120с таймауту — ставить ≥600с или фоновый процесс.
- Не править `catalog.go` руками (он генерируется; правка потеряется при следующем `go generate`).
- Если новый ключ не появился в `out.gotext.json` после шага 1 — инвокация неверная.
- При перезаписи `messages.gotext.json` целиком можно потерять «лишние» ключи — всегда merge.

## Открытый пункт (верифицирован на выполнении)
- Перевод вносится в `out.gotext.json` (заполняются пустые `"translation": ""`), затем **весь** `out.gotext.json` копируется в `messages.gotext.json`. Проверено: post-bake `out.gotext.json` содержит все переводы (пустых нет), копирование `cp out messages` выравнивает messages под актуальную экстракцию (65→62 ключа), каталог остаётся корректным.
