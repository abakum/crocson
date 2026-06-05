# План реализации: Улучшение обработки Android Intent через onNewIntent() в GoNativeActivity.java

## Контекст

Исходный план: `.kilo/plans/1780391027937-quiet-island.md`

### Ключевое наблюдение

`GoNativeActivity.java` **уже существует** в проекте (`/home/koka/src/crocson/GoNativeActivity.java`, 356 строк). Это не стандартный fyne-файл — он уже кастомизирован (есть `filePickerReturned`, `insetsChanged`, `keyboardTyped` и др. native-методы). Нужно просто добавить `onNewIntent()` и `processIntentData()`.

---

## Архитектурная диаграмма

### До (текущий подход)

```
┌─────────────────────────────────────────────────────────┐
│  Android OS                                             │
│                                                         │
│  Intent (SEND/VIEW/...)                                 │
│       │                                                 │
│       ▼                                                 │
│  GoNativeActivity (singleTop)                           │
│  └─ onCreate / onRestart ─── getIntent() ──┐            │
│       ❌ onNewIntent() НЕ переопределён      │            │
│                                            │            │
└────────────────────────────────────────────│────────────┘
                                             │
         lifecycle: SetOnEnteredForeground ◄─┘
                    │
    ┌───────────────▼──────────────────────────────────┐
    │  Каждый раз при входе в foreground:               │
    │                                                   │
    │  1. intentCancel()  ── остановка старой горутины  │
    │  2. intentWg.Wait() ── ожидание завершения        │
    │  3. drainChannel(uriFromIntent)   ── очистка      │
    │  4. drainChannel(textFromIntent)  ── очистка      │
    │  5. processIntent() ──► JNI/C код (580 строк)     │
    │  6. Запуск горутины с polling ticker (777ms)       │
    │                                                   │
    │  ┌─────────────────────────────────────────────┐  │
    │  │ process_intent_android.go (C/JNI, 626 строк)│  │
    │  │                                              │  │
    │  │  getIntent() через JNI                       │  │
    │  │       │                                      │  │
    │  │       ▼                                      │  │
    │  │  Парсинг action, flags, ClipData,            │  │
    │  │  STREAM extra, TEXT extra — всё в C          │  │
    │  │       │                                      │  │
    │  │       ▼                                      │  │
    │  │  receiveURIFromIntent() ──► uriFromIntent    │  │
    │  │  receiveTextFromIntent() ──► textFromIntent  │  │
    │  └─────────────────────────────────────────────┘  │
    │                                                   │
    │  Горутина-читатель:                               │
    │  ┌─────────────────────────────────────────────┐  │
    │  │  select {                                    │  │
    │  │    case <-ticker.C:                          │  │
    │  │      polling window handle (Android 9 hack)  │  │
    │  │      if handle changed → finish()+startAct() │  │
    │  │    case <-textFromIntent:                    │  │
    │  │      обработка текста                        │  │
    │  │    case <-uriFromIntent:                     │  │
    │  │      обработка URI                           │  │
    │  │  }                                           │  │
    │  └─────────────────────────────────────────────┘  │
    └───────────────────────────────────────────────────┘
```

**Проблемы**: getIntent() может вернуть старый интент | polling 777ms | 580 строк C/JNI | сложный lifecycle (cancel/wait/drain) | finish()+startActivity() для Android 9

### После (новый подход)

```
┌─────────────────────────────────────────────────────────┐
│  Android OS                                             │
│                                                         │
│  Intent (SEND/VIEW/...)     Lifecycle события           │
│       │                         │                       │
│       ▼                         ▼                       │
│  GoNativeActivity (singleTop)                           │
│  ┌───────────────────────────────────────────────────┐  │
│  │                                                   │  │
│  │  onNewIntent(intent):            ◄── НОВОЕ! ✅     │  │
│  │    setIntent(intent)                              │  │
│  │    processIntentData(intent)                      │  │
│  │                                                   │  │
│  │  onCreate():                                      │  │
│  │    processIntentData(getIntent()) ◄── начальный   │  │
│  │                                                   │  │
│  │  onResume():   → lifecycleEvent("resume")  ✅     │  │
│  │  onPause():    → lifecycleEvent("pause")   ✅     │  │
│  │  onStop():     → lifecycleEvent("stop")    ✅     │  │
│  │  onUserLeaveHint(): → lifecycleEvent("userLeave")✅│  │
│  │                                                   │  │
│  │  processIntentData(Intent intent):  (~60 строк)   │  │
│  │    фильтрация: null / MAIN / HISTORY / FRONT      │  │
│  │    парсинг ClipData, getData, STREAM, EXTRA_TEXT   │  │
│  │         │                                         │  │
│  │     есть данные? ──┬── ДА → intentReceived()      │  │
│  │                   └── НЕТ → intentCleared()       │  │
│  │                                                   │  │
│  │  native-методы (вызывают Go-код напрямую):        │  │
│  │    intentReceived(action, type, uris[], texts[])  │  │
│  │    intentCleared()                                │  │
│  │    lifecycleEvent(event)                          │  │
│  └───────────────────────────────────────────────────┘  │
│       │         │         │                              │
└───────│─────────│─────────│──────────────────────────────┘
        │         │         │  native calls (мгновенно, без polling)
        ▼         ▼         ▼
┌─────────────────────────────────────────────────────────┐
│  Go (process_intent_android.go, ~80 строк вместо 626)   │
│                                                         │
│  //export intentReceived()                              │
│  │  uriFromIntent  ← uri  (запись в канал)             │
│  │  textFromIntent ← text (запись в канал)             │
│  │                                                     │
│  //export intentCleared()                               │
│  │  textFromIntent ← ""  (сигнал "нет данных")         │
│  │                                                     │
│  //export lifecycleEvent()              ◄── НОВОЕ! ✅   │
│  │  lifecycleFromJava ← event  (запись в канал)        │
│  │                                                     │
│  Каналы ── мост между Android UI-потоком и Go-горутиной│
│  (НЕ заменять на прямые колбэки — см. риски)           │
└─────────────────────────────────────────────────────────┘
        │         │         │
        ▼         ▼         ▼
┌─────────────────────────────────────────────────────────┐
│  send.go — ЕДИНАЯ постоянная горутина                   │
│                                                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │  for {                                            │  │
│  │    select {                                       │  │
│  │      case event := <-lifecycleFromJava:      ✅   │  │
│  │        "resume" → notFinish=false, UI refresh    │  │
│  │        "pause"  → finish() если нужно            │  │
│  │        "stop"   → saveAccordionState()           │  │
│  │                                                   │  │
│  │      case text := <-textFromIntent:               │  │
│  │        обработка текста (addEntry, showPage)      │  │
│  │                                                   │  │
│  │      case uri := <-uriFromIntent:                 │  │
│  │        обработка URI (deepLink, copy, addEntry)   │  │
│  │    }                                              │  │
│  │  }                                                │  │
│  └───────────────────────────────────────────────────┘  │
│                                                         │
│  ❌ УБРАНО: intentCtx/intentCancel/intentWg             │
│  ❌ УБРАНО: drainChannel(uriFromIntent/textFromIntent)  │
│  ❌ УБРАНО: polling ticker 777ms                        │
│  ❌ УБРАНО: window handle tracking (oH/mH/nH)          │
│  ❌ УБРАНО: finish()+startActivity() для Android 9      │
│  ❌ УБРАНО: fyne lifecycle hooks для isAndroid ветки    │
└─────────────────────────────────────────────────────────┘
```

**Преимущества**: onNewIntent() — стандартный Android-паттерн | lifecycle из Java (работает на Android 9!) | мгновенная доставка | ~60 строк Java вместо 580 строк C | нет polling | единая горутина вместо пересоздания

---

## Почему Fyne lifecycle не работает на Android 9 (корневая причина)

### Анализ исходного кода Fyne (`android.go:434-504`)

Fyne **не использует** `onResume`/`onPause`/`onStart`/`onStop` Android Activity callbacks. Вместо этого Fyne выводит lifecycle stages из NativeWindow events:

| Fyne Stage | Что вызывает | Источник в `android.go` |
|---|---|---|
| `StageFocused` (= `SetOnEnteredForeground`) | `windowRedrawNeeded` — окно **перерисовывается** | строка 459 |
| `StageAlive` (= `SetOnExitedForeground`) | `windowDestroyed` — окно **уничтожается** | строка 484 |
| `StageDead` (= `SetOnStopped`) | `activityDestroyed` — активность уничтожена | строка 486 |

`onWindowFocusChanged` (строка 163) — **пустой**: `func onWindowFocusChanged(...) {}`

### Почему это ломается на Android 9

1. Пользователь нажимает **Home** или **Recents**
2. Android вызывает `onPause()`/`onStop()` на Java уровне — но Fyne эти callbacks **не обрабатывает**
3. **NativeWindow НЕ уничтожается** — Android 9 сохраняет его в памяти
4. `windowDestroyed` не срабатывает → `StageAlive` не отправляется → `SetOnExitedForeground` не вызывается
5. Пользователь возвращается в приложение
6. NativeWindow уже существует — не пересоздаётся, redraw не запрашивается
7. `windowRedrawNeeded` не срабатывает → `StageFocused` не отправляется → `SetOnEnteredForeground` не вызывается
8. **Итог: приложение зависло, ни один lifecycle hook не сработал**

### Почему это непреодолимо без модификации GoNativeActivity.java

NativeActivity предоставляет callbacks через `ANativeActivityCallbacks` (C-level). В этом API:
- ✅ Есть: `onNativeWindowCreated`, `onNativeWindowDestroyed`, `onNativeWindowRedrawNeeded`
- ❌ Нет: `onResume`, `onPause`, `onStart`, `onStop` — эти callbacks **не входят** в `ANativeActivityCallbacks`

Единственный способ получать `onResume`/`onPause` — переопределить их на **Java уровне** в `GoNativeActivity.java`. Fyne не может этого сделать без модификации этого файла.

### Наше решение

Переопределить `onResume()`/`onPause()`/`onStop()` в `GoNativeActivity.java` → вызывать native `lifecycleEvent()` → Go горутина обрабатывает через канал `lifecycleFromJava`. Это полностью обходит Fyne's window-event-based lifecycle.

---

## Шаги реализации

### 1. Модифицировать `GoNativeActivity.java`

#### 1a. Добавить native-методы (после строки 48)

```java
private native void intentReceived(String action, String type, String[] uris, String[] texts);
private native void intentCleared();
private native void lifecycleEvent(String event);
```

#### 1b. Переопределить lifecycle-методы Android (заменяют сломанные fyne hooks на Android 9)

```java
private static final String TAG = "croc";

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
public void onUserLeaveHint() {
    super.onUserLeaveHint();
    Log.d(TAG, "Java: onUserLeaveHint");
    lifecycleEvent("userLeave");
}
```

#### 1c. Добавить `onNewIntent()` override

```java
@Override
public void onNewIntent(Intent intent) {
    super.onNewIntent(intent);
    setIntent(intent);
    Log.d(TAG, "Java: onNewIntent action=" + (intent != null ? intent.getAction() : "null"));
    processIntentData(intent);
}
```

#### 1d. Добавить вызов `processIntentData()` в `onCreate()` (после `setupEntry()`)

```java
Log.d(TAG, "Java: onCreate — processing initial intent");
processIntentData(getIntent());
```

#### 1e. Добавить приватный метод `processIntentData()` — с полным логированием каждого этапа

```java
private void processIntentData(Intent intent) {
    if (intent == null) {
        Log.d(TAG, "Java: processIntentData — intent is null, calling intentCleared");
        intentCleared();
        return;
    }

    String action = intent.getAction();
    if (action == null || Intent.ACTION_MAIN.equals(action)) {
        Log.d(TAG, "Java: processIntentData — action=" + action + " (null or MAIN), calling intentCleared");
        intentCleared();
        return;
    }

    int flags = intent.getFlags();
    Log.d(TAG, "Java: processIntentData — action=" + action + " type=" + intent.getType() + " flags=0x" + Integer.toHexString(flags));

    if ((flags & 0x00100000) != 0) {
        Log.d(TAG, "Java: processIntentData — FLAG_LAUNCHED_FROM_HISTORY, calling intentCleared");
        intentCleared();
        return;
    }
    if ((flags & 0x00400000) != 0) {
        Log.d(TAG, "Java: processIntentData — FLAG_BROUGHT_TO_FRONT, calling intentCleared");
        intentCleared();
        return;
    }

    String type = intent.getType();
    ArrayList<String> uriList = new ArrayList<>();
    ArrayList<String> textList = new ArrayList<>();

    // ClipData
    ClipData clipData = intent.getClipData();
    if (clipData != null) {
        Log.d(TAG, "Java: ClipData found, itemCount=" + clipData.getItemCount());
        for (int i = 0; i < clipData.getItemCount(); i++) {
            ClipData.Item item = clipData.getItemAt(i);
            if (item.getUri() != null) {
                String uriStr = item.getUri().toString();
                Log.d(TAG, "Java: ClipData[" + i + "] uri=" + uriStr);
                uriList.add(uriStr);
            }
            if (item.getText() != null) {
                String textStr = item.getText().toString();
                if (textStr.length() > 100) {
                    Log.d(TAG, "Java: ClipData[" + i + "] text=" + textStr.substring(0, 100) + "...");
                } else {
                    Log.d(TAG, "Java: ClipData[" + i + "] text=" + textStr);
                }
                textList.add(textStr);
            }
        }
    } else {
        Log.d(TAG, "Java: ClipData is null");
    }

    // ACTION_VIEW — getData()
    if (uriList.isEmpty() && Intent.ACTION_VIEW.equals(action)) {
        Uri uri = intent.getData();
        if (uri != null) {
            Log.d(TAG, "Java: VIEW getData() uri=" + uri.toString());
            uriList.add(uri.toString());
        } else {
            Log.d(TAG, "Java: VIEW getData() is null");
        }
    }

    // ACTION_SEND / SEND_MULTIPLE — STREAM extra
    if (uriList.isEmpty() && (Intent.ACTION_SEND.equals(action) || Intent.ACTION_SEND_MULTIPLE.equals(action))) {
        if (Intent.ACTION_SEND_MULTIPLE.equals(action)) {
            ArrayList<Uri> uris = intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM);
            if (uris != null) {
                Log.d(TAG, "Java: SEND_MULTIPLE stream count=" + uris.size());
                for (Uri u : uris) {
                    Log.d(TAG, "Java: SEND_MULTIPLE uri=" + u.toString());
                    uriList.add(u.toString());
                }
            } else {
                Log.d(TAG, "Java: SEND_MULTIPLE stream is null");
            }
        } else {
            Uri stream = intent.getParcelableExtra(Intent.EXTRA_STREAM);
            if (stream != null) {
                Log.d(TAG, "Java: SEND stream uri=" + stream.toString());
                uriList.add(stream.toString());
            } else {
                Log.d(TAG, "Java: SEND stream is null");
            }
        }
    }

    // ACTION_SEND text/plain — EXTRA_TEXT
    if (textList.isEmpty() && Intent.ACTION_SEND.equals(action) && "text/plain".equals(type)) {
        String text = intent.getStringExtra(Intent.EXTRA_TEXT);
        if (text != null) {
            if (text.length() > 100) {
                Log.d(TAG, "Java: SEND EXTRA_TEXT=" + text.substring(0, 100) + "...");
            } else {
                Log.d(TAG, "Java: SEND EXTRA_TEXT=" + text);
            }
            textList.add(text);
        } else {
            Log.d(TAG, "Java: SEND EXTRA_TEXT is null");
        }
    }

    String[] urisArr = uriList.toArray(new String[0]);
    String[] textsArr = textList.toArray(new String[0]);

    Log.d(TAG, "Java: processIntentData result — uriCount=" + urisArr.length + " textCount=" + textsArr.length);
    intentReceived(action, type != null ? type : "", urisArr, textsArr);
}
```

Необходимые imports добавить: `android.content.ClipData`, `java.util.ArrayList`.

### 2. Заменить `process_intent_android.go` (626 строк → ~100 строк)

Заменить весь C/JNI код на Go callbacks, вызываемые из Java через native-методы.
Логирование на каждом этапе — как в старом JNI коде (`LogD` в C → `log.Debug`/`log.Error` в Go).

```go
//go:build android

// process_intent_android.go
// func processIntent(){}
// func intentReceived(){}  — вызывается из Java processIntentData()
// func intentCleared(){}   — вызывается из Java processIntentData()
// func lifecycleEvent(){}  — вызывается из Java onResume/onPause/onStop/onUserLeaveHint
package main

/*
#include <jni.h>
*/
import "C"

import (
	"unsafe"

	log "github.com/schollz/logger"
)

var lifecycleFromJava = make(chan string, 10)

//export intentReceived
func intentReceived(action *C.char, mimeType *C.char, uris **C.char, texts **C.char, uriCount C.jint, textCount C.jint) {
	goAction := C.GoString(action)
	goMime := C.GoString(mimeType)
	log.Debugf("Go: intentReceived action=%s mime=%s uriCount=%d textCount=%d", goAction, goMime, uriCount, textCount)

	goURIs := cStringArrayToSlice(uris, int(uriCount))
	goTexts := cStringArrayToSlice(texts, int(textCount))

	for i, uri := range goURIs {
		if len(uri) > 100 {
			log.Debugf("Go: intentReceived uri[%d]=%s...", i, uri[:100])
		} else {
			log.Debugf("Go: intentReceived uri[%d]=%s", i, uri)
		}
		select {
		case uriFromIntent <- uri:
		default:
			log.Errorf("Go: uriFromIntent channel full, dropped uri[%d]", i)
		}
	}
	for i, text := range goTexts {
		if len(text) > 100 {
			log.Debugf("Go: intentReceived text[%d]=%s...", i, text[:100])
		} else {
			log.Debugf("Go: intentReceived text[%d]=%s", i, text)
		}
		select {
		case textFromIntent <- text:
		default:
			log.Errorf("Go: textFromIntent channel full, dropped text[%d]", i)
		}
	}
}

//export intentCleared
func intentCleared() {
	log.Debug("Go: intentCleared")
	select {
	case textFromIntent <- "":
	default:
		log.Error("Go: textFromIntent channel full, dropped intentCleared")
	}
}

//export lifecycleEvent
func lifecycleEvent(event *C.char) {
	goEvent := C.GoString(event)
	log.Debugf("Go: lifecycleEvent %s", goEvent)
	select {
	case lifecycleFromJava <- goEvent:
	default:
		log.Errorf("Go: lifecycleFromJava channel full, dropped: %s", goEvent)
	}
}

func cStringArrayToSlice(arr **C.char, count int) []string {
	if count == 0 || arr == nil {
		log.Debugf("Go: cStringArrayToSlice count=%d arr=%v — returning nil", count, arr != nil)
		return nil
	}
	result := make([]string, count)
	start := unsafe.Pointer(arr)
	for i := 0; i < count; i++ {
		ptr := *(**C.char)(unsafe.Pointer(uintptr(start) + uintptr(i)*unsafe.Sizeof(arr)))
		result[i] = C.GoString(ptr)
	}
	log.Debugf("Go: cStringArrayToSlice decoded %d strings", count)
	return result
}

func processIntent() {
	log.Debug("Go: processIntent — no-op (intent delivery now via Java onNewIntent)")
}
```

**Ключевое**: `lifecycleFromJava` канал — мост между Java lifecycle callbacks и Go-горутиной. Заменяет сломанные на Android 9 fyne hooks.

### 3. Упростить lifecycle-обработку в `send.go` (строки 1319-1622)

**Ключевое изменение**: одна постоянная горутина слушает ТРИ источника через `select`:
1. `lifecycleFromJava` — события lifecycle из Java (заменяют сломанные fyne hooks)
2. `textFromIntent` — текст из `intentCleared()` / `intentReceived()`
3. `uriFromIntent` — URI из `intentReceived()`

Убрать из `SetOnEnteredForeground`:

1. **Polling ticker (777ms) и window handle tracking (`oH`/`mH`/`nH`)** — полностью убираем. Java lifecycle callbacks решают проблему Android 9
2. **`drainChannel(uriFromIntent)` / `drainChannel(textFromIntent)`** — не нужно очищать каналы
3. **`intentCtx` / `intentCancel` / `intentWg`** — горутина одна и постоянная, не пересоздаётся
4. **`processIntent()`** — пустая заглушка, вызов убрать
5. **fyne lifecycle hooks** (`SetOnExitedForeground`, `SetOnStopped`, `SetOnStarted`, `SetOnEnteredForeground`) — заменить на `lifecycleFromJava` канал

Предлагаемая упрощённая структура:

```go
if isAndroid {
    excludeRecents := false

    // Единая постоянная горутина — запускается один раз
    go func() {
        log.Debug("intent goroutine started")
        for {
            select {
            case <-appCtx.Done():
                log.Debug("intent goroutine stopped (appCtx)")
                return

            case event := <-lifecycleFromJava:
                log.Debugf("lifecycleFromJava: %s", event)
                switch event {
                case "resume":
                    notFinish = false
                    log.Debug("resume: notFinish=false")
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
                    log.Debugf("pause: notFinish=%v treeVisible=%v", notFinish, treeButton.Icon == theme.VisibilityIcon())
                    if !notFinish && treeButton.Icon == theme.VisibilityIcon() {
                        finish()
                    }

                case "stop":
                    log.Debug("stop: saving accordion state")
                    saveAccordionState()

                case "userLeave":
                    log.Debug("userLeave: user pressed Home or Recents")
                }

            case text := <-textFromIntent:
                if text == "" {
                    log.Debug("textFromIntent: empty (intentCleared) — setting notFinish=true")
                    notFinish = true
                    continue
                }
                if entry.Disabled() {
                    log.Debug("textFromIntent: entry disabled (sending), skipping")
                    continue
                }
                // ... существующая обработка текста (строки 1414-1437)

            case uriString := <-uriFromIntent:
                if uriString == "" {
                    log.Debug("uriFromIntent: empty, skipping")
                    continue
                }
                log.Debugf("uriFromIntent: %s", uriString)
                // ... существующая обработка URI (строки 1445-1597)
            }
        }
    }()
} else {
    // Для десктопа — оставить fyne lifecycle hooks как есть (строки 1623-1697)
    a.Lifecycle().SetOnEnteredForeground(func() { ... })
}
```

**Преимущества**:
- Горутина одна, живёт вечно — нет cancel/wait/drain
- Lifecycle приходит из Java — гарантированно работает на Android 9
- `finish()` вызывается из Go-горутины (не из Android UI потока) — корректный контекст для `driver.RunNative()`

### 4. Файлы для возможного удаления/упрощения

| Файл | Строк | Решение |
|------|-------|---------|
| `process_intent_android.go` | 626 | Заменить на ~80 строк Go callbacks + lifecycle |
| `finish_android.go` | 73 | Оставить — вызывается из горутины по `"pause"` событию |
| `start_activity_android.go` | 128 | Оставить — `start()` используется в `restart()` |
| `set_result_android.go` | 84 | Удалить — `setResult()` теперь вызывается из Java в `processIntentData()` |

**Примечание**: `setResult()` в Java-коде `processIntentData()` не вызывается напрямую. Нужно добавить вызов `setResult(RESULT_OK)` в `processIntentData()` когда есть валидные данные, и `setResult(RESULT_CANCELED)` когда нет. Это заменит вызовы `setResult()` из C-кода.

### 5. Обновить `for_android0.go`

Убедиться что заглушки `processIntent()`, `finish()`, `startActivity()` и `start()` остаются для `!android` сборки.

### 6. Неизменяемые файлы

- `AndroidManifest.xml` — `singleTop` уже установлен (строка 17), менять не нужно
- `intent.java` — справочник флагов, не влияет на компиляцию

---

## Порядок выполнения

1. **GoNativeActivity.java** — добавить `onNewIntent()`, `processIntentData()`, native-методы, импорты
2. **process_intent_android.go** — переписать: убрать C-код, оставить только Go callbacks
3. **send.go** — упростить lifecycle в `SetOnEnteredForeground`
4. **Тестирование** — убедиться что SEND, SEND_MULTIPLE, VIEW, текстовые интенты обрабатываются корректно

---

## Риски

1. **Передача C-строковых массивов через JNI** — нужно аккуратно работать с `String[]` из Java в Go через `**C.char`. Альтернатива: передавать данные по одному (как сейчас через каналы), но из Java native callback
2. **Архитектура: каналы, НЕ прямые колбэки** — `intentReceived()` вызывается из UI-потока Android, а каналы читаются в горутине Go. Каналы — правильный выбор, заменять на прямые колбэки нельзя: (а) обработка в колбэке заблокирует Android UI → ANR; (б) в колбэке нельзя делать copyFromURCProgress/addEntry/removeEntry/fyne.Do; (в) канал `uriFromIntent` пишется из двух источников — Java callback и `scannerIsBrowser` (send.go:1606); (г) канал буферизует быстрые последовательности интентов
3. **Поддержка fyne** — при обновлении fyne нужно проверять `GoNativeActivity.java` на совместимость. Но файл уже кастомный, так что риск минимален
4. **Android 9 workaround** — polling window handle полностью убирается. Lifecycle callbacks из Java (`onResume`/`onPause`/`onStop`) гарантированно работают на Android 9, в отличие от fyne lifecycle hooks
