# План: 16KB-выравнивание libcrocson.so (фикс в ../tools)

## Контекст
APK собираются через `fyne package --release`, AAB — через `fyne release`. Оба пути проходят через `mobile.RunNewBuild → goAndroidBuild(..., release=true)` в форке `../tools/cmd/fyne/internal/mobile`. ZIP-level выравнивание уже 16KB (`writer.go:145` align=16384). Проблема — **внутреннее выравнивание ELF LOAD-сегментов** `libcrocson.so`: сейчас `0x1000` (4KB), нужно `0x4000` (16KB).

## Найденная причина (в ../tools)
`build_androidapp.go:52-54`:
```go
if release { // Google Play Store requires 16K alignment
    env = []string{"CGO_LDFLAGS=\"-Wl,-z,max-page-size=16384\""}
}
```
Флаг **не доходит до линкера LLD** (по readelf .so = 4KB). Две причины:
1. **Литеральные двойные кавычки** внутри значения env → cgo видит токен `"-Wl,...` и отбрасывает его как невалидный.
2. **Нет `CGO_LDFLAGS_ALLOW`**: cgo по умолчанию отвергает `-Wl,-z,max-page-size` (его нет в safelist), нужен regex-override.

Проверено: `androidEnv[arch]` (`env.go:162`) **не** выставляет `CGO_LDFLAGS` → целевое значение не затирается; замены окружения через `cmd.Env = append([]string{}, env...)` (`build.go:340`) — этот же слайс несёт и ALLOW.

## Фикс (правка в ../tools)
**Файл:** `../tools/cmd/fyne/internal/mobile/build_androidapp.go`, строки 51-54.

Заменить:
```go
var env []string
if release { // Google Play Store requires 16K alignment
    env = []string{"CGO_LDFLAGS=\"-Wl,-z,max-page-size=16384\""}
}
```
на:
```go
var env []string
if release { // Google Play Store / Android 15+ require 16K ELF segment alignment
    env = []string{
        "CGO_LDFLAGS=-Wl,-z,max-page-size=16384",
        "CGO_LDFLAGS_ALLOW=-Wl,-z,max-page-size=[0-9]+",
    }
}
```
Ключевое: убрать кавычки вокруг значения + добавить `CGO_LDFLAGS_ALLOW`, чтобы cgo пропустил флаг на внешний линкер (LLD).

## Проверка
1. Пересобрать fyne CLI из форка:
   ```
   (cd ../tools/cmd/fyne && go install)
   ```
2. Пересобрать crocson APK локально (`make arm64`) И/ИЛИ дождаться CI.
3. Достать .so и проверить выравнивание:
   ```
   unzip -p crocson-arm64.apk lib/arm64-v8a/libcrocson.so > /tmp/lib.so
   readelf -lW /tmp/lib.so | grep LOAD
   ```
   **Критерий приёмки:** все LOAD-сегменты показывают `0x4000` (было `0x1000`).
4. Доп.: `apksigner verify --verbose crocson-arm64.apk` — подпись цела; `zipalign -c -v 4 ...` — ок.

## Если .so всё ещё 4KB (fallback)
- Собрать с трассировкой линковки: `fyne package -os android/arm64 --release` с `CGO`-детальным выводом (GOFLAGS `-x` через `-gobuildflags=-x` если поддерживается), убедиться, что в команде внешнего линкера присутствует `-z max-page-size=16384`.
- Альтернативный канал: передать через Go-линкер `go build -buildmode=c-shared -ldflags='-extldflags=-Wl,-z,max-page-size=16384'` (но тоже требует прохождения cgo-валидации).
- Крайний случай: пост-линковка `patchelf`/NDK-инструментом на `.so` в `tmpdir` перед упаковкой (вставить шаг после `goBuild`, `build_androidapp.go:133`) — переписать `PT_LOAD p_align` на 0x4000. Менее предпочтительно, т.к. не все тулзы корректно переписывают align.

## Дополнительно (гигиена, независимо)
- Локальный `make arm64` подписывает дефолтным ключом `CN=gomobile`. Привести к релизному ключу `croc` (`-keystore`/`-key` в Makefile) — на краш не влияет, но обеспечит идентичность local/CI.
- 16KB-фикс остаётся **гипотезой** причины крача на Huawei до получения их logcat; но дефект 4KB-align реален и подлежит исправлению. После пересборки — повторить Cloud Testing / запросить logcat в заявке D620968.

## Порядок выполнения
1. Правка `build_androidapp.go` (фикс env).
2. `go install` fyne CLI из форка.
3. Пересобрать crocson apk, `readelf` → `0x4000`.
4. Прогнать на Huawei Cloud Testing / ответить в заявку D620968.
