# Этап 3: Intent handling через `onNewIntent()` — одна горутина для lifecycle + intents

## Цель

Заменить архитектуру обработки интентов: вместо одноразовой intent-горутины (пересоздание на каждый resume + `processIntent()` через `driver.RunNative`) → **одна постоянная горутина**, читающая lifecycle-события И intent-данные из каналов. Парсинг интентов переносится из C в Java (`GoNativeActivity.java`).

## Текущая архитектура (что меняем)

```
Java: lifecycle events → lifecycleFromJava channel → lifecycle goroutine (send.go:1564)
                                                            ↓ "resume"
                                                      handleResume():
                                                        - intentCancel + intentWg.Wait (убить старую горутину)
                                                        - drainChannel (очистить каналы)
                                                        - запустить НОВУЮ intent-горутину
                                                        - processIntent() → driver.RunNative → C.processIntent → getIntent() → парсинг → receiveURI/Text
```

- `process_intent_android.go`: ~580 строк C-кода для парсинга интентов через JNI
- `handleResume` closure (строки 1325-1562): пересоздаёт intent-горутину при каждом resume
- `intentCancel/intentWg/drainChannel`: управление жизненным циклом одноразовой горутины

## Новая архитектура

```
Java: processIntentData(intent) → intentURI()/intentText() native callbacks
                                        ↓
                                   C bridge (intent_android.c)
                                        ↓
                                   Go //export → uriFromIntent/textFromIntent channels
                                        ↓
                                   ОДНА постоянная горутина (select):
                                     - lifecycleFromJava → pause/stop/resume
                                     - uriFromIntent     → обработка URI
                                     - textFromIntent    → обработка text
```

- Парсинг интентов — в Java (чистый, типобезопасный, ~60 строк вместо ~580 строк C)
- Одна горутина на всё время жизни приложения
- Нет `intentCancel/intentWg/drainChannel/handleResume`
- `processIntent()` больше не вызывается из Go

## Изменения по файлам

### 1. `GoNativeActivity.java` — добавить `onNewIntent()` + `processIntentData()`

**Новые native-объявления** (после строки 50):

```java
private native void intentURI(String uri);
private native void intentText(String text);
```

**Новый метод `processIntentData`** (в конец класса, перед `}`):

```java
private void processIntentData(Intent intent) {
    if (intent == null || intent.getAction() == null || Intent.ACTION_MAIN.equals(intent.getAction())) {
        return;
    }

    String action = intent.getAction();
    String type = intent.getType();
    boolean hasData = false;

    // 1. ClipData (SEND, SEND_MULTIPLE, VIEW с clip)
    ClipData clipData = intent.getClipData();
    if (clipData != null) {
        for (int i = 0; i < clipData.getItemCount(); i++) {
            ClipData.Item item = clipData.getItemAt(i);
            if (item.getUri() != null) {
                intentURI(item.getUri().toString());
                hasData = true;
            }
            if (item.getText() != null) {
                intentText(item.getText().toString());
                hasData = true;
            }
        }
        }
        return;
    }

    // 2. SEND text/plain → EXTRA_TEXT
    if (Intent.ACTION_SEND.equals(action) && "text/plain".equals(type)) {
        String text = intent.getStringExtra(Intent.EXTRA_TEXT);
        if (text != null) {
            intentText(text);
            return;
        }
    }

    // 3. VIEW → getData()
    if (Intent.ACTION_VIEW.equals(action)) {
        Uri uri = intent.getData();
        if (uri != null) {
            intentURI(uri.toString());
            return;
        }
    }

    // 4. SEND → EXTRA_STREAM
    if (Intent.ACTION_SEND.equals(action)) {
        Uri stream = intent.getParcelableExtra(Intent.EXTRA_STREAM);
        if (stream != null) {
            intentURI(stream.toString());
            return;
        }
    }

    // 5. SEND_MULTIPLE → ArrayList<Uri>
    if (Intent.ACTION_SEND_MULTIPLE.equals(action)) {
        ArrayList<Uri> uris = intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM);
        if (uris != null && !uris.isEmpty()) {
            for (Uri u : uris) {
                intentURI(u.toString());
            }
            return;
        }
    }
}
```

Логика порядка обработки — та же что в C-коде (`process_intent_android.go:96-578`): ClipData → text/plain → VIEW → SEND STREAM → SEND_MULTIPLE.

**`onNewIntent()`** — новый override (после `onDestroy`):

```java
@Override
protected void onNewIntent(Intent intent) {
    super.onNewIntent(intent);
    setIntent(intent);
    Log.d(TAG, "Java: onNewIntent action=" + (intent != null ? intent.getAction() : "null"));
    processIntentData(intent);
}
```

**`onCreate()`** — обработка начального интента только при свежем запуске (после `lifecycleEvent("create")`, строка 245):

```java
if (savedInstanceState == null) {
    processIntentData(getIntent());
}
```

`savedInstanceState == null` → свежий запуск. `!= null` → пересоздание из Недавних после убийства процесса — пропускаем.

### 2. `intent_android.c` — НОВЫЙ файл, JNI-bridge

```c
#include "_cgo_export.h"
#include "crocson_jni.h"

static jboolean intentCaseException(JNIEnv* env, const char* context) {
    if ((*env)->ExceptionCheck(env)) {
        LogE("Exception in intent: %s", context);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        return JNI_TRUE;
    }
    return JNI_FALSE;
}

JNIEXPORT void Java_org_golang_app_GoNativeActivity_intentURI(JNIEnv *env, jobject thiz, jstring uri) {
    if (uri == NULL) return;
    const char *curi = (*env)->GetStringUTFChars(env, uri, NULL);
    if (intentCaseException(env, "GetStringUTFChars") || curi == NULL) return;
    intentURINotify((char*)curi);
    (*env)->ReleaseStringUTFChars(env, uri, curi);
}

JNIEXPORT void Java_org_golang_app_GoNativeActivity_intentText(JNIEnv *env, jobject thiz, jstring text) {
    if (text == NULL) return;
    const char *ctext = (*env)->GetStringUTFChars(env, text, NULL);
    if (intentCaseException(env, "GetStringUTFChars") || ctext == NULL) return;
    intentTextNotify((char*)ctext);
    (*env)->ReleaseStringUTFChars(env, text, ctext);
}
```

### 3. `intent_android.go` — НОВЫЙ файл, `//export` функции

```go
//go:build android

package main

/*
*/
import "C"

//export intentURINotify
func intentURINotify(uri *C.char) {
	if uri != nil {
		goURI := C.GoString(uri)
		LogD("intent: URI " + goURI)
		select {
		case uriFromIntent <- goURI:
		default:
			LogD("intent: URI channel full, dropping")
		}
	}
}

//export intentTextNotify
func intentTextNotify(text *C.char) {
	if text != nil {
		goText := C.GoString(text)
		LogD("intent: text received")
		select {
		case textFromIntent <- goText:
		default:
			LogD("intent: text channel full, dropping")
		}
	}
}
```

### 4. `send.go` — заменить `if isAndroid { ... }` блок (строки 1319-1577)

**Удалить**:
- `intentWg`, `intentCtx`, `intentCancel` — переменные управления одноразовой горутиной
- `handleResume` closure — вся функция (строки 1325-1562)
- Вызовы `drainChannel()` — не нужны без пересоздания горутины

**Добавить** — одна постоянная горутина:

```go
if isAndroid {
    go func() {
        for {
            select {
            case event := <-lifecycleFromJava:
                switch event {
                case "resume":
                    notFinish = false
                    if scannerIsBrowser {
                        clipboardText := a.Clipboard().Content()
                        if clipboardText != clipboardBeforeScan && strings.HasPrefix(clipboardText, IO) {
                            log.Debugf("scannerIsBrowser: sending clipboard to uriFromIntent: %q", clipboardText)
                            uriFromIntent <- clipboardText
                        }
                        scannerIsBrowser = false
                    }
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
                case "permissionDialog":
                    notFinish = true
                }

            case text := <-textFromIntent:
                if text == "" {
                    log.Debug("doneProcessIntent notFinish")
                    notFinish = true
                    continue
                }
                if entry.Disabled() {
                    log.Debug("doneProcessIntent Sending")
                    continue
                }
                log.Debugf("clip\n%s", text)
                src := join(hashToFilename(text))
                if fe := addEntry(src, func(d *widget.Button, p *widget.ProgressBar, l *widget.Label) {
                    setSizes(p, int64(len(text)))
                }); fe == nil {
                    continue
                }

                source, err := os.Create(src)
                if err != nil {
                    log.Errorf("create: %v", err)
                    continue
                }

                _, err = source.WriteString(text)
                if err != nil {
                    source.Close()
                    os.Remove(src)
                    log.Errorf("write: %v", err)
                    continue
                }

                source.Close()
                showPage()

            case uriString := <-uriFromIntent:
                // ... ТА ЖЕ обработка URI что сейчас в intent-горутине (строки 1385-1543)
                // Все `return` заменены на `continue`
                // БЛОК HasStoragePermission УБРАН — Java проверяет права до отправки URI
            }
        }
    }()
} else {
    // не-Android: без изменений
}
```

**Ключевые изменения в обработчике URI** (перенос из старой intent-горутины):
- Все `return` (выход из одноразовой горутины) → `continue` (следующая итерация select)
- Вложенные `go func()` не меняются — `return` внутри них выходит из вложенной горутины
- Текст обработки `deepLink`, `isDAV`, копирования файлов — **без изменений**, только `return` → `continue`

### 5. Удалить `process_intent_android.go`

Файл полностью удаляется. Его содержимое заменено:
- C-код парсинга интентов → Java `processIntentData()`
- `receiveURIFromIntent`/`receiveTextFromIntent` → `intentURINotify`/`intentTextNotify` в `intent_android.go`
- `processIntent()` Go-функция → больше не вызывается из Go

### 6. `for_android0.go` — убрать заглушку

Удалить `func processIntent() {}` (строка 20) — функция больше не существует.

## Что сохранено БЕЗ изменений

| Что | Где |
|---|---|
| Каналы `uriFromIntent`/`textFromIntent` (100 буфер) | `main.go:111-112` |
| Логика обработки text (создание файла из текста) | `send.go` text handler |
| Логика обработки URI (deepLink, isDAV, копирование) | `send.go` URI handler |
| `scannerIsBrowser` обработка | `send.go` "resume" handler |
| `notFinish` логика | `send.go` "resume"/"pause"/text handler |
| Lifecycle C-bridge (`lifecycle_android.c`) | Без изменений |
| Lifecycle Go (`lifecycle_android.go`) | Без изменений |
| Не-Android блок (`} else {`) | Без изменений |
| `saveAccordionState()`, `finish()` | Без изменений |
| `drainChannel()` функция | Остаётся (может использоваться в других местах) |

## Сценарии

### Запуск из лаунчера (MAIN)
```
Java: onCreate(savedInstanceState=null) → processIntentData(getIntent()) → MAIN → return (ничего не делаем)
Java: onStart → onResume → "resume"
Go: notFinish=false, fyne.Do(...)
```

### Запуск через Share (SEND URI)
```
Java: onCreate → processIntentData(getIntent()) → SEND → intentURI("content://...")
Go: URI handler → копирование файла
Java: onResume → "resume"
Go: notFinish=false, fyne.Do(...)
```

### Новый интент пока приложение активно (singleTop)
```
Java: onNewIntent(intent) → processIntentData(intent) → intentURI(...)
Go: URI handler → обработка
Java: onResume → "resume"
Go: notFinish=false, fyne.Do(...)
```

### Возврат из background без нового интента
```
Java: onRestart → onStart → onResume → "resume"
Go: notFinish=false, fyne.Do(...) — БЕЗ обработки интента
```

### scannerIsBrowser (QR-сканер в браузере)
```
Java: onResume → "resume"
Go: notFinish=false, scannerIsBrowser=true → clipboard → uriFromIntent → URI handler
```

### Ожидание ответа на запрос разрешений — в Java

Проверка и запрос разрешений происходит в Java **до** отправки URI в Go. Go получает только данные, готовые к обработке.

Android lifecycle: `requestPermissions()` → `onPause` → юзер нажимает → `onRequestPermissionsResult()` → `onResume`

```
1. Java: onCreate → processIntentData() → file:// URI → нет прав → сохранить pending, requestPermissions()
2. Java: permission dialog → onPause → "pause"
3. Go: pause → notFinish=false → finish()... ← ПРОБЛЕМА!
```

**Проблема**: при паузе для permission dialog, `notFinish=false` → `finish()` убьёт активность.
**Решение**: в `processIntentData()` перед `requestPermissions()` вызвать `lifecycleEvent("permissionDialog")`,
а в Go на `"permissionDialog"` установить `notFinish=true`.

Итоговый поток:
```
1. Java: processIntentData() → file:// URI → checkSelfPermission() ≠ GRANTED
2. Java: lifecycleEvent("permissionDialog") → Go: notFinish=true
3. Java: requestPermissions()
4. Java: permission dialog → onPause → "pause"
5. Go: pause → !notFinish=false → finish() НЕ вызывается ✓
6. Java: юзер даёт разрешение → onRequestPermissionsResult(true) → intentURI() → Go channel
7. Java: onResume → "resume"
8. Go: resume → notFinish=false
9. Go: select → uriFromIntent → HasStoragePermission()=true → файл копируется ✓
```

Если разрешение **не дали**: `onRequestPermissionsResult()` не вызывает `intentURI()` → URI не попадает в Go.

#### Реализация в `GoNativeActivity.java`

Добавить поле и override:

```java
private ArrayList<String> pendingIntentURIs = null;

@Override
public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
    super.onRequestPermissionsResult(requestCode, permissions, grantResults);
    if (requestCode == 123 && pendingIntentURIs != null) {
        boolean granted = true;
        for (int result : grantResults) {
            if (result != 0) granted = false;
        }
        Log.d(TAG, "Java: permissionResult granted=" + granted);
        if (granted) {
            for (String uri : pendingIntentURIs) {
                intentURI(uri);
            }
        }
        pendingIntentURIs = null;
    }
}
```

Изменить `processIntentData()` — перед отправкой URI проверить разрешение:

```java
// В конце processIntentData(), перед отправкой URI в Go:
if (!uriList.isEmpty()) {
    boolean needsPermission = false;
    for (String uri : uriList) {
        if (uri.startsWith("file://")) {
            needsPermission = true;
            break;
        }
    }
    if (needsPermission && checkSelfPermission("android.permission.READ_EXTERNAL_STORAGE") != 0) {
        pendingIntentURIs = uriList;
        lifecycleEvent("permissionDialog");
        requestPermissions(new String[]{
            "android.permission.READ_EXTERNAL_STORAGE",
            "android.permission.WRITE_EXTERNAL_STORAGE"
        }, 123);
        return;
    }
    for (String uri : uriList) {
        intentURI(uri);
    }
}
```

**Не нужны**: `pendingFileURI` в Go, `permissionResultChan`, `permissionResultNotify`, JNI-bridge для permission result.

**Единственное изменение в Go** — добавить case `"permissionDialog"` в switch:

```go
case "permissionDialog":
    notFinish = true
```

## Сводка

| Файл | Действие |
|---|---|
| `GoNativeActivity.java` | +`onNewIntent()`, +`processIntentData()`, +`onRequestPermissionsResult()`, +permission check, +2 native declarations |
| `intent_android.c` | **НОВЫЙ** — JNI bridge (2 функции) |
| `intent_android.go` | **НОВЫЙ** — `//export` функции записи в каналы |
| `send.go` | Заменить блок `if isAndroid` — одна горутина, убрать `HasStoragePermission`/`RequestStoragePermission` из URI handler |
| `process_intent_android.go` | **УДАЛИТЬ** |
| `for_android0.go` | Удалить `func processIntent() {}` |

---

# Этап 4: Удалить `POST_NOTIFICATIONS` из манифеста

## Обоснование

`POST_NOTIFICATIONS` (API 33+) — единственное оставшееся опасное разрешение в манифесте. Оно нужно только для обычных уведомлений (`NotificationManager.notify()`). Foreground service уведомления (`startForeground()`) **освобождены** от этого разрешения.

Единственный потребитель `showCrocNotification()` — `start_activity_android.go:125` (уведомление "App closed. Tap to start." после `finish()`). На Android 13+ без `POST_NOTIFICATIONS` оно просто не появится — приложение всё равно можно открыть через иконку.

## Изменения

| Файл | Действие |
|---|---|
| `AndroidManifest.xml:111` | Удалить строку `<uses-permission android:name="android.permission.POST_NOTIFICATIONS" />` |

## Что НЕ меняется

- `notification_android.go` — остаётся как есть (код не сломается, уведомления просто молча не покажутся на API 33+)
- `CrocsonService.java` — foreground уведомление продолжает работать без `POST_NOTIFICATIONS`
- `showCrocNotification()` — вызов в `start_activity_android.go:125` остаётся (на API <33 работает как раньше, на API 33+ — молча игнорируется системой)

---

# Этап 5: Bugfix — дублирование intent при возврате из Recent Apps (Total Commander)

## Описание бага

1. Total Commander отправляет файл → intent обработан, файл в корзине
2. Удалить файл из корзины
3. Нажать Recent Apps → выбрать приложение
4. **Баг:** тот же файл опять попадает в корзину

**Примечание:** Другие файловые менеджеры используют `FLAG_ACTIVITY_NEW_TASK | FLAG_ACTIVITY_CLEAR_TASK` → приложение НЕ попадает в Recents. Это баг Total Commander, но мы можем защититься.

### Лог (isTaskRoot() не работает)

```
# Первый запуск
06-04 01:26:35.726 Java: onCreate isTaskRoot=true, initialIntentProcessed=false
06-04 01:26:35.726 Java: onCreate processing intent  ✅

# Возврат из Recents
06-04 01:26:53.792 Java: onCreate isTaskRoot=true, initialIntentProcessed=true
06-04 01:26:53.826 Java: onCreate fresh launch, resetting flag  ← ПРОБЛЕМА!
06-04 01:26:53.826 Java: onCreate processing intent  ❌ Дубликат
```

**Почему `isTaskRoot=true` при возврате из Recents?**

При возврате из Recent Apps Android уничтожает **задачу целиком**, а не только активность. Новая активность — корень новой задачи → `isTaskRoot=true`.

## Решение: восстановить проверку флагов интента (как в старом C-коде)

Старый C-код проверял флаги интента:
```c
if (flags & 0x00100000) { // LAUNCHED_FROM_HISTORY
    LogD("C: Skipping: Activity launched from history");
    goto cleanup;
}
if (flags & 0x00400000) { // BROUGHT_TO_FRONT
    LogD("C: Skipping: Activity brought to front");
    goto cleanup;
}
```

В Java:
- `Intent.FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY` = 0x00100000 — активность запущена из Recents
- `Intent.FLAG_ACTIVITY_BROUGHT_TO_FRONT` = 0x00400000 — активность вынесена вперёд

Проверяем эти флаги в `onCreate` — если есть любой из них, пропускаем обработку.

### Логика

| Сценарий | Флаги | Обработать intent? |
|---|---|---|
| Свежий запуск из лаунчера | Ни одного | ✅ Да |
| Свежий запуск из файлового менеджера (SEND) | Ни одного | ✅ Да |
| Возврат из Recents | `LAUNCHED_FROM_HISTORY` | ❌ Нет (дубликат) |
| Возврат из background | `BROUGHT_TO_FRONT` | ❌ Нет (дубликат) |
| Конфигурация changed | (не важно) | ❌ Нет (по savedInstanceState) |
| App running, file sent | N/A (onNewIntent) | ✅ Да (onNewIntent обрабатывает отдельно) |

### Реализация в `GoNativeActivity.java:onCreate`

**Убрать:**
- Статическое поле `initialIntentProcessed`
- `isTaskRoot()` проверку
- Любые SharedPreferences

**Заменить onCreate на:**
```java
@Override
public void onCreate(Bundle savedInstanceState) {
    load();
    super.onCreate(savedInstanceState);
    setupEntry();
    updateTheme(getResources().getConfiguration());
    Log.d(TAG, "Java: onCreate");
    lifecycleEvent("create");

    Intent intent = getIntent();
    int flags = (intent != null) ? intent.getFlags() : 0;
    boolean fromHistory = (flags & Intent.FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY) != 0;
    boolean broughtToFront = (flags & Intent.FLAG_ACTIVITY_BROUGHT_TO_FRONT) != 0;

    Log.d(TAG, "Java: onCreate flags=" + flags + ", LAUNCHED_FROM_HISTORY=" + fromHistory +
          ", BROUGHT_TO_FRONT=" + broughtToFront + ", savedInstanceState=" +
          (savedInstanceState == null ? "null" : "not null"));

    if (savedInstanceState == null && !fromHistory && !broughtToFront) {
        // Fresh launch from launcher or file manager
        Log.d(TAG, "Java: onCreate processing intent (fresh launch)");
        processIntentData(intent);
    } else {
        if (fromHistory) {
            Log.d(TAG, "Java: onCreate skipping intent (LAUNCHED_FROM_HISTORY=true)");
        } else if (broughtToFront) {
            Log.d(TAG, "Java: onCreate skipping intent (BROUGHT_TO_FRONT=true)");
        } else {
            Log.d(TAG, "Java: onCreate skipping intent (savedInstanceState != null, config change)");
        }
    }
```
    }

    // ... обработка текста ...
```

### НЕ нужно в Java

- Убрать `initialIntentProcessed` статическое поле
- Убрать `isTaskRoot()` проверку
- Убрать SharedPreferences
- Логировать intent данные для отладки

### Логирование для отладки

В Go можно добавить:
```go
func isDuplicateURI(uri string) bool {
    // ... проверки ...
    if exists && timeSince < uriDuplicateWindow {
        log.Debugf("Duplicate URI skipped: %s (processed %v ago)", uri, timeSince)
        return true
    }
    log.Debugf("Processing URI: %s (new or old)", uri)
    lastProcessedURIs[uri] = time.Now().UnixNano()
    return false
}
```

---

# Статус разрешений в манифесте

| Разрешение | Тип | Когда нужно | Статус |
|---|---|---|---|
| `INTERNET` | Normal | Всегда | ✅ Без изменений |
| `WRITE_EXTERNAL_STORAGE` | Dangerous (API < 30) | file:// URI из других приложений, Download в `/sdcard/Downloads/` (API < 29) | ✅ Обрабатывается в Java (`sendIntentURIs()`) и C (`download_android.go:654`) |
| `READ_EXTERNAL_STORAGE` | Dangerous (API < 30) | file:// URI из других приложений, Download в `/sdcard/Downloads/` (API < 29) | ✅ Обрабатывается в Java и C |
| `WAKE_LOCK` | Normal | Пробуждение устройства для WebDAV | ✅ Без изменений |
| `FOREGROUND_SERVICE` | Normal | Foreground service для WebDAV | ✅ Без изменений |
| `FOREGROUND_SERVICE_DATA_SYNC` | Normal (API 34+) | Тип foreground service | ✅ Без изменений |
| ~~`POST_NOTIFICATIONS`~~ | Dangerous (API 33+) | ~~Обычные уведомления~~ | ~~Удалён~~ |

---

## SAF файлпикеры — НЕ требуют разрешений

```java
// GoNativeActivity.java:175-204
ACTION_OPEN_DOCUMENT       // открыть файл
ACTION_OPEN_DOCUMENT_TREE  // открыть папку (FLAG_GRANT_READ_URI_PERMISSION)
ACTION_CREATE_DOCUMENT     // сохранить файл
```

- **URI permissions выдаются автоматически** через системный пикер
- Android 5+ (API 21+)
- Доступ только к выбранным файлам/папкам через URI
- В манифесте только `<queries>` (строки 71-104) — проверка доступности

## file:// URI из других приложений

Пользователь может скинуть `file://` URI через Share → `sendIntentURIs()` проверяет и запрашивает разрешения в Java.

## Download в Downloads

`download_android.go:654` — создаёт файлы в `/sdcard/Downloads/` на API < 29. На API 29+ используется MediaStore API — без разрешений.
