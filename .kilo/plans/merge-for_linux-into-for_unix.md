# Слияние for_linux в for_unix (отказ от BSD как target)

## Решение
BSD исключается из поддерживаемых платформ сборки. Это снимает единственное препятствие
к объединению linux-файлов с unix-файлами: тег `!linux || android` отличался от
`!(unix && !android && !darwin)` **только** ради покрытия BSD (non-linux unix).

После слияния BSD формально попадёт под real-реализации из `for_unix.go`, но т.к. BSD больше
не собирается (ни локально, ни в CI), это допустимо.

## Текущее состояние (после переименования в прошлом шаге)
| файл            | тег                              | содержимое                              |
|-----------------|----------------------------------|-----------------------------------------|
| `for_unix.go`   | `unix && !android && !darwin`    | caffeinate, registerScheme(real) и др.  |
| `for_unix0.go`  | `!(unix && !android && !darwin)` | stub `registerScheme`                   |
| `for_linux.go`  | `linux && !android`              | `ensureDesktopEntry`(real), `isAppImage`|
| `for_linux0.go` | `!linux || android`              | stub `ensureDesktopEntry`               |

## Целевое состояние
| файл           | тег                              | содержимое                                       |
|----------------|----------------------------------|--------------------------------------------------|
| `for_unix.go`  | `unix && !android && !darwin`    | + `ensureDesktopEntry`(real), `isAppImage`       |
| `for_unix0.go` | `!(unix && !android && !darwin)` | + stub `ensureDesktopEntry`                      |
| `for_linux.go` | удалён                           |                                                  |
| `for_linux0.go`| удалён                           |                                                  |

## Изменения

### 1. `for_unix.go` — дополнить
Добавить импорты, которых нет в текущем import-блоке:
`path/filepath`, `github.com/BurntSushi/toml`, `github.com/adrg/xdg`.
(Текущий блок: bytes, os, os/exec, strconv, strings, sync, sync/atomic, fyne, logger.)

Перенести из `for_linux.go` функции `ensureDesktopEntry()` и `isAppImage(exe string) bool`
без изменений в теле.

Проверка зависимостей (все определены в пакете, доступны из любого файла):
- `ID`        — main.go:58
- `iconData`  — main.go:102
- `fyneApp`   — about.go:28
- `FyneApp`   — about.go:167

### 2. `for_unix0.go` — дополнить
Добавить строку stub (тег и package без изменений):
```go
func ensureDesktopEntry() {}
```
(Импортов не требуется — тело пустое, как и у текущего `registerScheme`.)

### 3. Удалить `for_linux.go` и `for_linux0.go`
После переноса содержимого файлы становятся лишними:
- `git rm for_linux.go for_linux0.go`

## Почему теги можно не менять
Платформы, которые реально собираются:
- **linux** (native, CI build-linux, `make` таргеты) → real-реализации из `for_unix.go`.
- **windows / darwin / android** → stub-и из `for_unix0.go`.

Этим множествам в точности соответствуют текущие теги `for_unix.go` / `for_unix0.go`.
BSD сюда не входит, поэтому сужать тег `for_unix.go` до `linux && !android` необязательно
и лишь добавило бы churn (rename + retag). Оставляем теги как есть.

## Поведение
Без изменений для всех поддерживаемых платформ:
- linux: `ensureDesktopEntry` и `registerScheme` — real (как и раньше).
- windows/darwin/android: оба — stub (как и раньше).

## Проверка
- `go build ./...`   — linux, real-реализации из for_unix.go.
- `go vet ./...`     — статический анализ.
- windows/darwin: проверяются через CI (`make wsl` / darwin-джоб) — поведение stub не изменилось.
- BSD: больше не проверяется и не является target.
