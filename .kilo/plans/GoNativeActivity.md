# План: Перехват и логгирование жизненного цикла Android через GoNativeActivity.java

## Цель

Освоить расширение `GoNativeActivity.java` на примере перехвата lifecycle-событий Android (`onCreate`, `onStart`, `onResume`, `onPause`, `onStop`, `onDestroy`, `onRestart`) и логгирования их в logcat через native-вызовы в Go.

## Существующие паттерны логгирования JNI в проекте

### 1. C-уровень (внутри cgo `/* ... */` блоков)

Все android-файлы crocson используют один и тот же паттерн:

```c
#include <android/log.h>
#define LogD(...) __android_log_print(ANDROID_LOG_DEBUG, "croc", __VA_ARGS__)
#define LogE(...) __android_log_print(ANDROID_LOG_ERROR, "croc", __VA_ARGS__)
```

Тег лога: `"croc"` → фильтр в logcat: `adb logcat | grep "croc    :"`

### 2. Go-уровень — `log_android.go`

```go
//go:build android
package main

/*
#include <android/log.h>
void LogD(const char* message) {
    __android_log_write(ANDROID_LOG_DEBUG, "croc", message);
}
*/
import "C"

func LogD(message string) {
    cmessage := C.CString(message)
    defer C.free(unsafe.Pointer(cmessage))
    C.LogD(cmessage)
}
```

### 3. Java-уровень — Fyne использует тег `"Fyne"`

```java
Log.e("Fyne", "loadLibrary android.app.lib_name failed", e);
```

В GoNativeActivity.java и android.c тег = `"Fyne"`. В crocson тег = `"croc"`.

### 4. Паттерн `log.go` (GUI writer)

`logWriter.Write()` пишет в GUI-буфер + вызывает `LogD(line)` для дублирования в logcat.

## Архитектура: как native-методы из GoNativeActivity попадают в Go

**Ключевой факт**: native-методы `GoNativeActivity.java` (объявленные как `private native void ...`) **НЕ** проходят через `android.c` из fyne. Они резолвятся через стандартный JNI механизм:

```
Java: private native void lifecycleEvent(String event)
                ↓
JNI автоматически ищет: Java_org_golang_app_GoNativeActivity_lifecycleEvent
                ↓
Но мы НЕ можем использовать такой naming, потому что:
  - мы не можем изменить android.c (он внутри fyne)
  - мы не можем добавить //export в cgo блок другого файла
```

**Правильный подход** для crocson — использовать **`//export`** в cgo-блоке нового Go-файла:

```
GoNativeActivity.java
  private native void lifecycleEvent(String event)
       ↓ (JNI naming convention)
  Java_org_golang_app_GoNativeActivity_lifecycleEvent(JNIEnv*, jobject, jstring)
       ↓ (определена в C-блоке Go-файла)
  Вызывает Go-функцию через export
```

НО! `android.c` из fyne уже определяет `Java_org_golang_app_GoNativeActivity_*` функции для 6 существующих native-методов. **Добавить новые функции с этим prefix нельзя** — будет конфликт линковки.

**Решение**: использовать **другой native-метод** и C-wrapper в cgo-блоке:

### Вариант A: Новый Go-файл с `//export` (невозможно для JNI naming)

`//export` в Go создаёт символ с точным именем функции, но JNI требует формат `Java_пакет_Класс_метод`. Дефисы в имени недопустимы для Go-функций.

### Вариант B: Регистрация через `RegisterNatives` в `JNI_OnLoad` (невозможно — fyne уже определяет JNI_OnLoad)

### Вариант C (выбранный): C-wrapper в cgo-блоке + `Log.d()` из Java

Вместо нового native-метода — логгировать **прямо в Java** через `android.util.Log`, а Go уведомлять через существующий механизм (`driver.RunNative`). Но это не даёт перехвата lifecycle на Java-уровне.

### Вариант D (выбранный): Новый native-метод + JNI-bridge через cgo C-код

Fyne's `android.c` регистрирует JNI-функции для `Java_org_golang_app_GoNativeActivity_*`. Но мы можем определить JNI-функции для **новых** native-методов в нашем собственном cgo C-блоке — **если они не конфликтуют** с существующими.

**Проверка**: Fyne's `android.c` определяет функции для: `filePickerReturned`, `insetsChanged`, `keyboardTyped`, `keyboardDelete`, `backPressed`, `setDarkMode`. Новые методы (`lifecycleEvent`) — **не конфликтуют**.

**Проблема**: C-функция `Java_org_golang_app_GoNativeActivity_lifecycleEvent` должна быть определена в C-блоке. Но cgo требует `//export` для экспорта Go-функций, а JNI naming несовместим с `//export` (подчёркивания в имени — это не проблема, проблема в том, что `//export` не работает для JNI naming с пакетом `org.golang.app`).

**Итого**: JNI-функцию `Java_org_golang_app_GoNativeActivity_lifecycleEvent` нужно определить **в C-секции cgo** (не как `//export` Go-функцию), а внутри неё вызывать Go-функцию через `extern` или через Go callback.

### Финальный вариант (работающий)

```
GoNativeActivity.java:
  private native void lifecycleEvent(String event);
       ↓ JNI
  Java_org_golang_app_GoNativeActivity_lifecycleEvent  (определена в C-блоке lifecycle_android.go)
       ↓ вызывает Go
  lifecycleEventFromJava(event string)                  (Go-функция в lifecycle_android.go)
       ↓
  log.Debug("lifecycle: " + event)                     (видно в GUI + logcat)
```

## План реализации

### Шаг 1: Создать `lifecycle_android.go`

Новый файл `//go:build android` с cgo C-блоком, содержащим:
- JNI-функцию `Java_org_golang_app_GoNativeActivity_lifecycleEvent`
- Вспомогательный макрос `LogD` (как в других файлах)

Go-часть:
- `lifecycleEventFromJava(event string)` — логгирует через `log.Debug` (видно в GUI + logcat)

### Шаг 2: Модифицировать `GoNativeActivity.java`

Добавить:
1. Объявление native-метода: `private native void lifecycleEvent(String event);`
2. Константу `private static final String TAG = "croc";`
3. Override lifecycle-методов с логированием через `Log.d(TAG, ...)` + вызов native `lifecycleEvent()`:
   - `onCreate` — лог + `lifecycleEvent("create")` (в существующий `onCreate`, после `setupEntry()`)
   - `onStart` — новый override
   - `onResume` — новый override
   - `onPause` — новый override
   - `onStop` — новый override
   - `onDestroy` — новый override
   - `onRestart` — новый override

### Шаг 3: Нестандартные вызовы для `for_android0.go`

В `for_android0.go` (заглушка для `!android`) уже есть `func finish() {}`. Нужно добавить заглушку и для нового кода, но в данном случае весь новый код в файле с `//go:build android`, так что заглушка не нужна.

## Детальная реализация

### `lifecycle_android.go` (новый файл)

```go
//go:build android

package main

/*
#include <jni.h>
#include <android/log.h>
#include <stdlib.h>

#define LogD(...) __android_log_print(ANDROID_LOG_DEBUG, "croc", __VA_ARGS__)

void lifecycleEventFromGo(const char* event);

JNIEXPORT void Java_org_golang_app_GoNativeActivity_lifecycleEvent(JNIEnv *env, jobject thiz, jstring event) {
    const char *cevent = (*env)->GetStringUTFChars(env, event, NULL);
    LogD("JNI lifecycleEvent: %s", cevent);
    lifecycleEventFromGo(cevent);
    (*env)->ReleaseStringUTFChars(env, event, cevent);
}
*/
import "C"

import "unsafe"

func init() {
    // Регистрация не нужна — JNI найдёт функцию по имени
}

//export lifecycleEventFromGo
func lifecycleEventFromGo(event *C.char) {
    goEvent := C.GoString(event)
    logLifecycle(goEvent)
}

func logLifecycle(event string) {
    log.Debugf("lifecycle: %s", event)
}
```

**Проблема**: `log` из `github.com/schollz/logger` — глобальная переменная, которая инициализируется в `main()`. При вызове из `onCreate` (до завершения `main()`) `log` может быть не готов.

**Решение**: Использовать прямой `LogD()` из `log_android.go` для надёжного логирования из JNI callback:

```go
func logLifecycle(event string) {
    LogD("lifecycle: " + event)
}
```

### `GoNativeActivity.java` — изменения

#### Добавить после строки 48 (после `private native void setDarkMode(boolean dark);`):

```java
private native void lifecycleEvent(String event);
```

#### Добавить после строки 31 (class-level):

```java
private static final String TAG = "croc";
```

#### Добавить lifecycle overrides (после `updateTheme`, перед закрывающей `}`):

```java
@Override
protected void onStart() {
    super.onStart();
    Log.d(TAG, "Java: onStart");
    lifecycleEvent("start");
}

@Override
protected void onRestart() {
    super.onRestart();
    Log.d(TAG, "Java: onRestart");
    lifecycleEvent("restart");
}

@Override
protected void onResume() {
    super.onResume();
    Log.d(TAG, "Java: onResume");
    lifecycleEvent("resume");
}

@Override
protected void onPause() {
    super.onPause();
    Log.d(TAG, "Java: onPause");
    lifecycleEvent("pause");
}

@Override
protected void onStop() {
    super.onStop();
    Log.d(TAG, "Java: onStop");
    lifecycleEvent("stop");
}

@Override
protected void onDestroy() {
    super.onDestroy();
    Log.d(TAG, "Java: onDestroy");
    lifecycleEvent("destroy");
}
```

#### В существующем `onCreate()` — добавить лог (после `setupEntry()`, строка ~240):

```java
Log.d(TAG, "Java: onCreate");
lifecycleEvent("create");
```

## Что будет видно в logcat

```
adb logcat | grep -E "croc|Fyne"

D/croc    : JNI lifecycleEvent: create
D/croc    : lifecycle: create
D/Fyne    : ... (стандартные сообщения fyne)
D/croc    : JNI lifecycleEvent: start
D/croc    : lifecycle: start
D/croc    : JNI lifecycleEvent: resume
D/croc    : lifecycle: resume
...
```

## Риски и ограничения

1. **Конфликт имён JNI**: Fyne's `android.c` уже определяет `Java_org_golang_app_GoNativeActivity_*` для 6 методов. Наше новое имя `lifecycleEvent` **не конфликтует**.

2. **Время вызова**: `onCreate` вызывается **до** завершения `main.main()`. Go runtime уже инициализирован (c-shared buildmode), но `log` из `schollz/logger` может быть не готов. Используем `LogD()` (прямой `__android_log_write`) — он работает всегда.

3. **Поддержка при обновлении fyne**: `GoNativeActivity.java` — кастомный файл, не обновляется автоматически. При обновлении fyne нужно сравнивать с оригиналом.

4. **cgo `//export` и JNI**: Go-функция `lifecycleEventFromGo` помечена `//export`, но вызывается из C-кода **внутри того же cgo-блока**. Это корректно — `//export` создаёт символ в итоговом .so.

## Исправление: дублирование логов

Логируется в двух местах:
1. `lifecycle_android.c:10` — `LogD("lifecycle: %s", cevent)` (C-level)
2. `lifecycle_android.go:9` — `LogD("lifecycle: " + C.GoString(event))` (Go-level)

**Исправление**: убрать `LogD` из `lifecycle_android.c`, оставить только Go-level логирование.

## Обработка исключений

### Существующий паттерн в проекте

Все C-файлы с JNI-кодом определяют локальную `caseException()`:
```c
static jboolean caseException(JNIEnv* env, const char* context) {
    if ((*env)->ExceptionCheck(env)) {
        LogD("Exception in %s", context);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        return JNI_TRUE;
    }
    return JNI_FALSE;
}
```
Используется после каждого JNI-вызова: `if (caseException(env, "GetStringUTFChars")) return;`

Функция дублируется в `caffeinate_android.go`, `process_intent_android.go`, `documents_android.go`, `finish_android.go` и др.

### Что нужно для `lifecycle_android.c`

Текущий код минимален:
```c
const char *cevent = (*env)->GetStringUTFChars(env, event, NULL);
if (cevent == NULL) return;
lifecycleEventNotify((char*)cevent);
(*env)->ReleaseStringUTFChars(env, event, cevent);
```

`GetStringUTFChars` — единственный JNI-вызов. Он возвращает `NULL` при OOM или если JVM бросает исключение. Текущая проверка `if (cevent == NULL) return;` достаточна, но не логирует причину.

**План**:
1. Добавить `caseException()` в `lifecycle_android.c` (как в других C-файлах проекта)
2. Использовать её после `GetStringUTFChars` для логирования исключений
3. Java-код (`GoNativeActivity.java`) не нуждается в try/catch — lifecycle-методы вызывают `super.*()` + `lifecycleEvent()`, исключения в native-методе уже обработаны в C

### Итоговый `lifecycle_android.c`

```c
#include "_cgo_export.h"
#include <jni.h>
#include <android/log.h>

#define LogD(...) __android_log_print(ANDROID_LOG_DEBUG, "croc", __VA_ARGS__)

static jboolean caseException(JNIEnv* env, const char* context) {
    if ((*env)->ExceptionCheck(env)) {
        LogD("Exception in lifecycleEvent: %s", context);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        return JNI_TRUE;
    }
    return JNI_FALSE;
}

JNIEXPORT void Java_org_golang_app_GoNativeActivity_lifecycleEvent(JNIEnv *env, jobject thiz, jstring event) {
    if (event == NULL) {
        LogD("lifecycleEvent: event is NULL");
        return;
    }
    const char *cevent = (*env)->GetStringUTFChars(env, event, NULL);
    if (caseException(env, "GetStringUTFChars") || cevent == NULL) return;
    lifecycleEventNotify((char*)cevent);
    (*env)->ReleaseStringUTFChars(env, event, cevent);
}
```

## Этап 2: Замена Fyne lifecycle callbacks на наши (из GoNativeActivity)

### Цель

Заменить Fyne's `SetOnEnteredForeground`/`SetOnExitedForeground`/`SetOnStopped`/`SetOnStarted` на наши `lifecycleEvent` callbacks из GoNativeActivity.java. Это устранит:
- Android 9 hack с polling window handle каждые 777ms (строки 1377-1401)
- `finish()+startActivity()` для восстановления после зависания
- Пересоздание горутины (`intentCancel/intentWg/drainChannel`) при каждом входе в foreground
- Зависание при возврате из background на Android 9

Интенты остаются как есть (`processIntent()` через `driver.RunNative`) — это этап 3.

### Что заменяем

**Сейчас** (`send.go:1319-1622`, блок `if isAndroid`):

```
Fyne SetOnEnteredForeground → intentCancel + intentWg.Wait + drain + processIntent + новая горутина + ticker 777ms
Fyne SetOnExitedForeground → finish() если !notFinish
Fyne SetOnStopped          → saveAccordionState()
Fyne SetOnStarted          → oH = wHandle(w)  [для Android 9 hack]
ticker 777ms               → отслеживание смены window handle → finish()+startActivity()
```

**После**:

```
lifecycleFromJava "resume"  → intentCancel + intentWg.Wait + drain + processIntent + новая горутина (БЕЗ ticker)
lifecycleFromJava "pause"   → finish() если !notFinish
lifecycleFromJava "stop"    → saveAccordionState()
lifecycleFromJava "create"/"start"/"restart"/"destroy" → только лог
```

### Что УБИРАЕМ

1. **Android 9 polling hack** (строки 1377-1401): `ticker 777ms`, `oH/mH/nH`, `finish()+startActivity()`
2. **Fyne lifecycle hooks**: `SetOnEnteredForeground/SetOnExitedForeground/SetOnStopped/SetOnStarted` для Android
3. Переменные `oH`, `mH`

### Что ОСТАВЛЯЕМ без изменений

1. **Механизм пересоздания горутины**: `intentCancel/intentWg/drainChannel` — горутина одноразовая (делает `return` после обработки интента), её нужно пересоздавать при каждом `resume`
2. **`processIntent()`** — вызывается при `resume`, пишет в `uriFromIntent/textFromIntent`
3. **Горутина с `textFromIntent/uriFromIntent`** — та же, но без ticker 777ms
4. **Блок `} else {`** (строка 1623) — для не-Android, Fyne hooks без изменений

### Реализация

#### 1. Добавить канал `lifecycleFromJava` в `send.go` или новый файл

```go
var lifecycleFromJava = make(chan string, 10)
```

#### 2. Обновить `lifecycleEventNotify` в `lifecycle_android.go`

```go
//export lifecycleEventNotify
func lifecycleEventNotify(event *C.char) {
    goEvent := C.GoString(event)
    LogD("lifecycle: " + goEvent)
    select {
    case lifecycleFromJava <- goEvent:
    default:
        LogD("lifecycle: channel full, dropping " + goEvent)
    }
}
```

#### 3. Переписать Android lifecycle блок в `send.go` (строки 1319-1622)

Заменить весь `if isAndroid { ... }` блок. Логика пересоздания горутины сохраняется, но триггер теперь от `lifecycleFromJava` вместо Fyne hooks, и убран Android 9 hack:

```go
if isAndroid {
    var intentWg sync.WaitGroup
    intentCtx, intentCancel := context.WithCancel(appCtx)

    go func() {
        for event := range lifecycleFromJava {
            switch event {
            case "resume":
                notFinish = false

                intentCancel()
                intentWg.Wait()
                drainChannel(uriFromIntent)
                drainChannel(textFromIntent)
                intentCtx, intentCancel = context.WithCancel(appCtx)

                intentWg.Add(1)
                go func() {
                    defer intentWg.Done()
                    for {
                        select {
                        case <-intentCtx.Done():
                            log.Debug("intent goroutine stopped")
                            return
                        case text := <-textFromIntent:
                            // ... та же обработка text БЕЗ изменений
                        case uriString := <-uriFromIntent:
                            // ... та же обработка URI БЕЗ изменений
                        }
                    }
                }()

                processIntent()
                fyne.Do(func() {
                    at.OnSelected(at.Selected())
                    de.Bounce(ti.Content.Refresh)
                })

            case "pause":
                if !notFinish && treeButton.Icon == theme.VisibilityIcon() {
                    finish()
                }

            case "stop":
                saveAccordionState()
            }
        }
    }()
} else {
    // не-Android: Fyne lifecycle hooks (без изменений)
    ...
}
```

**Что изменилось**:
- Триггер: `range lifecycleFromJava` вместо `a.Lifecycle().SetOnEnteredForeground(...)` — работает на Android 9!
- УБРАНО: `ticker 777ms`, `oH/mH/nH`, `finish()+startActivity()`
- СОХРАНЕНО: `intentCancel/intentWg/drainChannel` — горутина одноразовая, пересоздаётся при каждом `resume`
- СОХРАНЕНО: обработка `textFromIntent/uriFromIntent` внутри горутины — без изменений
- `processIntent()` вызывается после запуска новой горутины, как раньше

#### 4. Переменные `oH`, `mH` — удалить

Больше не нужны без Android 9 hack. Переменные объявлены на строках 1325-1326.

#### 5. `drainChannel` — оставить

Функция используется при пересоздании горутины (пункт 3). Не удаляем.

### Риски

1. **Порядок событий**: `lifecycleEvent("resume")` приходит из Java-потока, `processIntent()` делает JNI-вызовы через `driver.RunNative`. Нужно убедиться что JNI готов в момент resume. Fyne уже обрабатывает `onResume` в `android.c` (обновляет контекст), так что к моменту нашего `resume` JNI контекст актуален.

2. **Горутина vs UI-поток**: `textFromIntent/uriFromIntent` обработка может обновлять UI. Уже обёрнуто в `fyne.Do()` где нужно — проверим.

3. **Обратная совместимость**: не-Android код (блок `else`) не меняется.

## Этап 2: Выполнено ✅

### Дата: 2026-06-03

### Что сделано

Заменены Fyne lifecycle callbacks (`SetOnEnteredForeground`/`SetOnExitedForeground`/`SetOnStopped`/`SetOnStarted`) на события из `lifecycleFromJava` канала (Java→C→Go).

#### Изменённые файлы

| Файл | Изменение |
|---|---|
| `send.go` (строки 1319-1577) | Блок `if isAndroid` переписан |
| `lifecycle_android.go` | `lifecycleFromJava` канал + `lifecycleEventNotify` уже были готовы из этапа 1 |

#### Что удалено

1. **Android 9 polling hack**: `ticker 777ms`, переменные `oH`/`mH`/`nH`, `finish()+startActivity()` восстановление
2. **Fyne lifecycle hooks** для Android: `SetOnEnteredForeground`, `SetOnExitedForeground`, `SetOnStopped`, `SetOnStarted`
3. **`excludeRecents`**: переменная и все использования (оба branch в `SetOnExitedForeground` делали `finish()` — мёртвый код)

#### Что добавлено

1. **`handleResume` closure** — заменяет `SetOnEnteredForeground` callback. Содержит всю логику пересоздания intent-горутины + `processIntent()` + обновление UI через `fyne.Do()`. Использование closure вместо прямого вложения в switch/case позволяет сохранить indentation intent-горутины без изменений (+300 строк кода не потребовалось переформатировать).
2. **Lifecycle goroutine** — `go func() { for event := range lifecycleFromJava { switch event { ... } } }()`:
   - `"resume"` → `handleResume()`
   - `"pause"` → `finish()` если `!notFinish && treeButton.Icon == theme.VisibilityIcon()`
   - `"stop"` → `saveAccordionState()`
3. **`fyne.Do()`** wrapper для `at.OnSelected()` и `de.Bounce()` — эти вызовы теперь в lifecycle goroutine, а не в Fyne UI goroutine

#### Что сохранено без изменений

- Механизм пересоздания intent-горутины: `intentCancel/intentWg.Wait()/drainChannel`
- Вся обработка `textFromIntent/uriFromIntent` внутри intent-горутины
- `processIntent()` вызов
- `scannerIsBrowser` обработка
- Блок `} else {` для не-Android (Fyne hooks без изменений)

### Результаты тестирования

#### Android 16 (эмулятор arm64) ✅

```
create → start → resume                         — запуск из лаунчера
pause → finish → stop → destroy                 — уход в background
create → start → resume                         — возврат (новая активность)
pause (нет finish) → stop → destroy             — из Недавних (LAUNCHED_FROM_HISTORY, не finish)
onRestart → start → resume                      — возврат без destroy
SEND intent → create → start → resume           — share файла (content:// URI)
pause → stop → onRestart → start → resume       — выход из chooser и возврат
```

#### Android 9 (реальное устройство, API 28) ✅

```
create → start → resume                         — запуск из лаунчера
pause → stop                                    — открытие браузера (WebDAV)
restart → start → resume + processIntent        — возврат из браузера
pause → stop                                    — повторный уход
restart → start → resume + processIntent        — возврат
pause → stop → destroy                          — уход с уничтожением
create → start → resume + SEND intent           — share APK файла
pause (permission dialog) → resume + повторная обработка — запрос разрешений
pause → finish → stop → destroy                 — нормальное завершение
create → start → resume + LAUNCHED_FROM_HISTORY — возврат из Недавних
```

**Ключевой вывод**: На Android 9 Java lifecycle events (`onPause`/`onStop`/`onResume`) срабатывают корректно, в отличие от Fyne hooks которые иногда пропускали эти события. Android 9 polling hack (ticker 777ms) больше не нужен.

## Этап 3: Intent handling refactor (планируется)

### Цель

Заменить вызов `processIntent()` через `driver.RunNative` на `onNewIntent()` в GoNativeActivity.java, чтобы новые интенты обрабатывались без пересоздания активности.

### Статус: не начат
