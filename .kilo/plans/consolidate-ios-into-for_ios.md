# Консолидация ios-кода по образцу for_android (.c/.h/.go)

## Идея (по аналогии с for_android)
Android выносит нативный код из cgo-preamble `.go`-файла в отдельные `for_android.h` (прототипы)
и `for_android.c` (реализация), а `for_android.go` держит только Go-обёртки с минимальным
preamble `#include "for_android.h"`. `for_android0.go` НЕ существует — non-android сторона
реализована в `for_unix.go`/`for_darwin.go`/`for_windows.go` и `child_desktop.go`.

Для ios делаем то же, но с расширением `.m` (Objective-C), т.к. ios-код на ObjC
(`@autoreleasepool`, `[NSArray …]`, UIKit). cgo автоматически компилирует файлы `.m` как ObjC.

Важно: **`for_ios0.go` не создаём.** Non-ios сторона не состоит из заглушек — у каждой функции
есть своя реальная реализация на других платформах:

| Функция (из ios-файлов)             | где ещё есть real-реализация                       |
|--------------------------------------|----------------------------------------------------|
| `caffeinate` / `SleepAllowed`        | for_unix.go, for_darwin.go, for_windows.go, for_android.go |
| `Child` / `ChildDownload`            | for_android.go, child_desktop.go                   |
| `CreateBookmarkFromURL` и др. (5 шт) | только ios, вне iOS не вызываются → stub не нужен  |

Создать `for_ios0.go` с тегом `!ios` значило бы задублировать/перекрыть существующие реализации
на android/darwin/windows/desktop → ошибка компиляции.

## Источники (3 файла, тег `//go:build ios`)
- `caffeinate_ios.go` — `setIdleTimerDisabled`, Go: `caffeinate`, `SleepAllowed`
- `download_ios.go` — ObjC: `CreateBookmarkFromURLDownload`, `CreateFileInDownloads`; Go: те же + `ChildDownload`
- `child_ios.go` — ObjC: `CreateBookmarkFromURL`, `ResolveBookmarkToURL`, `StopAccessingSecurityScopedResource`, `CreateFileInTreeIOS`; Go: те же + `CreateFileInTree`, `Child`

## Изменения

### 1. Создать `for_ios.h` (прототипы всех нативных функций)
```c
#ifndef FOR_IOS_H
#define FOR_IOS_H

#import <Foundation/Foundation.h>   // для BOOL, NSURL, NSData, NSError и т.п.

void setIdleTimerDisabled(BOOL disabled);

char* CreateBookmarkFromURLDownload(void);
char* CreateFileInDownloads(char* bookmarkDataStr, char* fileName, char* mimeType);

char* CreateBookmarkFromURL(const char* urlString);
char* ResolveBookmarkToURL(const char* bookmarkDataString, bool* isStaleOut);
void  StopAccessingSecurityScopedResource(const char* urlString);
char* CreateFileInTreeIOS(const char* bookmarkData, const char* fileName, const char* mimeType);

#endif
```
`#import <Foundation/Foundation.h>` в начале — чтобы `BOOL`/`YES`/`NO` были видны и в Go-preamble
(который `#include "for_ios.h"`), и в `.m`. Это работает т.к. текущие ios-файлы уже импортируют
Foundation в preamble и компилируются под iOS.

### 2. Создать `for_ios.m` (объединённая ObjC-реализация)
Слить bodies всех 6 функций из preamble-блоков трёх файлов, добавив вверху:
```objective-c
#import <Foundation/Foundation.h>
#import <UIKit/UIKit.h>          // для caffeinate: UIApplication
#import <stdlib.h>               // strdup
#import "for_ios.h"
```
Имена/сигнатуры функций оставить 1:1 как сейчас (Go-сторона зовёт их по тем же именам).

### 3. Создать `for_ios.go` (тег `//go:build ios`, вся Go-логика)
Минимальный cgo-preamble + слив Go-кода из трёх файлов:
```go
//go:build ios

package main

/*
#include <stdlib.h>
#include "for_ios.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/storage"
	log "github.com/schollz/logger"
)
```
Перенести Go-функции без изменений тел:
- `caffeinate`, `SleepAllowed`
- `CreateBookmarkFromURLDownload`, `CreateFileInDownloads`, `ChildDownload`
- `CreateBookmarkFromURL`, `ResolveBookmarkToURL`, `StopAccessingSecurityScopedResource`, `CreateFileInTree`, `Child`

Импорты дедуплицировать (`errors`, `fmt`, `strings`, `unsafe`, `fyne`, `driver`, `storage`, `log`,
`sync/atomic`). `C.YES`/`C.NO`/`C.CString`/`C.GoString`/`C.free` продолжат работать (встроенные cgo
хелперы + макросы ObjC из Foundation).

### 4. Удалить старые файлы
- `caffeinate_ios.go`
- `download_ios.go`
- `child_ios.go`

## Итоговая тройка (зеркало for_android)
| for_android            | for_ios          |
|------------------------|------------------|
| `for_android.h`        | `for_ios.h`      |
| `for_android.c` (C/JNI)| `for_ios.m` (ObjC)|
| `for_android.go`       | `for_ios.go`     |

## Поведение
Без изменений. На iOS собирается тот же набор функций; сигнатуры и тела идентичны текущим.
На остальных платформах ничего не меняется (их файлы не трогаются).

## Проверка / риски
- ❗ В этой среде нет iOS/darwin тулчейна — `GOOS=ios` и даже `GOOS=darwin go build` здесь не собрать
  (падение на go-gl/CGO). Локально верифицировать ios-сборку нельзя.
- ✅ Что можно проверить локально: `go vet ./...` и сборка linux-варианта (для контроля, что не задели
  общие имена / нет конфликта символов на linux).
- Финальная верификация iOS — на машине с Xcode:
  `fyne package -os ios --release` (или через `make`/CI при наличии iOS-раннера).
  В текущем `.github/workflows/fyne.yml` iOS не собирается (есть windows/linux/macos/android),
  поэтому проверить CI-прогоном нельзя — только вручную.
- Основной риск — корректность слияния ObjC-кода в `.m` и состава `#import`/прототипов в `.h`.
  Поэтому после переноса важно прогнать реальную iOS-сборку перед коммитом релиза.
