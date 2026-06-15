# Переименование/реорганизация linux-файлов

## Контекст
В проекте уже сложилось соглашение по парам build-файлов:
- `for_unix.go`   → real impl  (`unix && !android && !darwin`)
- `for_unix0.go`  → stub       (`!(unix && !android && !darwin)`)

Для linux пока есть только `install_desktop_linux.go` (real, `linux && !android`) и
`install_desktop_other.go` (stub, `!linux || android`). Приводим их к тому же виду.

### Есть ли ещё код для линукса?
Нет. Поиск по build-тегам `linux`/`!linux` находит только два файла выше.
Остальная unix-специфика (caffeinate, systemd-inhibit, registerScheme) лежит в `for_unix.go`
и intentionally покрывает весь `unix && !android && !darwin`, включая BSD.

`install_desktop_linux.go` содержит 2 функции: `ensureDesktopEntry` (real) и
вспомогательную `isAppImage` (используется только внутри ensureDesktopEntry). Обе переезжают вместе.
`ensureDesktopEntry` вызывается из `main.go:310`.

## Изменения

### 1. Переименовать `install_desktop_linux.go` → `for_linux.go`
- Содержимое (build-тег `linux && !android`, обе функции) без изменений.
- Это просто `git mv` (или удаление старого + создание нового).

### 2. Переименовать `install_desktop_other.go` → `for_linux0.go`
- Содержимое переносим в новый файл:
```go
//go:build !linux || android

package main

func ensureDesktopEntry() {}
```
- Тег `!linux || android` оставляем как есть — это complement к `linux && !android`,
  что гарантирует наличие заглушки (или real) для любой платформы, **включая BSD**.
  (Менять тег на complement от `for_unix0.go` нельзя — сломает сборку на BSD.)

### 3. Удалить старые файлы
- `install_desktop_linux.go` и `install_desktop_other.go` удаляются после создания `for_linux.go`/`for_linux0.go`.

## Итоговая структура пар
| real                    | stub                  |
|-------------------------|-----------------------|
| `for_unix.go`           | `for_unix0.go`        |
| `for_linux.go`          | `for_linux0.go`       |

## Проверка
> Важно: прямая кросс-сборка вида `GOOS=windows go build ./...` падает на зависимости
> `github.com/go-gl/gl/v2.1/gl` — это **досуществующая** проблема отсутствия CGO/OpenGL-тулчейна
> (подтверждено: тот же fail на исходном коде до переименования). К переименованию отношения не имеет.

Реальные таргеты сборки (через Makefile, тулчейн mingw для windows):
- `go build ./...`        — linux (real impl из for_linux.go)
- `go vet ./...`          — статический анализ
- `make wsl`              — windows-сборка через mingw:
  `GOOS=windows CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 go build -ldflags=-s`
  (обеспечивает CGO/OpenGL для fyne/go-gl)
- `make windowsgui`       — windows с `-H windowsgui` (опционально)
- `make darwin`           — macOS (опционально)
