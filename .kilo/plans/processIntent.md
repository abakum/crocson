# План: Улучшение обработки Android Intent через модификацию GoNativeActivity.java

## Как это сделано сейчас (обходной путь)

### Архитектура

Поскольку `GoNativeActivity.java` (из `fyne.io/fyne/v2/internal/driver/mobile/app/`) нельзя было менять, вся обработка интентов реализована через JNI/C из Go-кода:

1. **AndroidManifest.xml** (стр. 17): `launchMode="singleTop"` (выбран вынужденно, как единственный способ отследить получение интента без `onNewIntent()`) + intent-filters для `SEND`, `SEND_MULTIPLE`, `VIEW`, `MAIN`
2. **send.go** (стр. 1355-1622): В `SetOnEnteredForeground` lifecycle hook каждый раз при входе в foreground:
   - Останавливается старая горутина обработки (`intentCancel()` + `intentWg.Wait()`)
   - Очищаются каналы (`drainChannel`)
   - Запускается новая горутина с polling ticker'ом (777ms) для:
     - Чтения из каналов `uriFromIntent` / `textFromIntent`
     - Отслеживания смены window handle (костыль для Android 9)
   - Вызывается `processIntent()`
3. **process_intent_android.go** (~626 строк): C/JNI код, который:
   - Получает activity через `driver.RunNative()`
   - Вызывает `getIntent()` через JNI
   - Парсит action, flags, ClipData, STREAM extra, TEXT extra
   - Отправляет данные в Go-каналы через `receiveURIFromIntent()` / `receiveTextFromIntent()`
4. **finish_android.go** / **start_activity_android.go**: Костыль `finish()` + `startActivity()` для перезапуска активности

### Проблемы текущего подхода

1. **`getIntent()` вместо `onNewIntent()`**: Из-за `singleTop` Android вызывает `onNewIntent()`, но `GoNativeActivity.java` его не переопределяет. Вместо этого код дергает `getIntent()` в `SetOnEnteredForeground`, что может возвращать *старый* интент
2. **`singleTop` как вынужденный выбор**: launchMode выбран не потому что он идеален, а потому что без `onNewIntent()` это был единственный способ отследить приход нового интента (активность не пересоздаётся, можно снова вызвать `getIntent()` в lifecycle hook)
2. **Polling 777ms**: Горутина с ticker'ом опрашивает window handle для детекции зависаний — это hack и ресурсоёмко
3. **Сложное управление lifecycle**: `intentCtx`, `intentWg`, `intentCancel`, `drainChannel` — хрупкая конструкция из-за того что обработка интентов идёт через lifecycle hook, а не через callback
4. **~580 строк C/JNI кода**: Парсинг ClipData, STREAM extra, SEND_MULTIPLE в C — громоздко и подвержено ошибкам
5. **`finish()` + `startActivity()` танец**: Для обработки edge cases (Android 9 зависание, exclude from recents) — визуальное мигание

---

## Предлагаемый новый подход

### Ключевое изменение: добавить `onNewIntent()` в `GoNativeActivity.java`

Переопределить `onNewIntent()` в Java, чтобы Android напрямую доставлял новый интент в работающую активность. Теперь `singleTop` становится не вынужденным костылём, а правильным архитектурным выбором — Android гарантирует вызов `onNewIntent()` вместо пересоздания активности. При желании можно рассмотреть и другие launchMode (`singleTask`), поскольку `onNewIntent()` работает с любым из них.

### Шаги реализации

#### 1. Создать кастомный `GoNativeActivity.java`

добавить:

```java
// Новые native-методы
private native void intentReceived(String action, String type, String[] uris, String[] texts);
private native void intentCleared();

@Override
public void onNewIntent(Intent intent) {
    super.onNewIntent(intent);
    setIntent(intent);  // обновляем getIntent()
    processIntentData(intent);
}

@Override
protected void onCreate(Bundle savedInstanceState) {
    // ... существующий код ...
    // Обработка начального интента
    processIntentData(getIntent());
}

private void processIntentData(Intent intent) {
    if (intent == null) {
        intentCleared();
        return;
    }

    String action = intent.getAction();
    if (action == null || action.equals(Intent.ACTION_MAIN)) {
        intentCleared();
        return;
    }

    // Проверяем флаги LAUNCHED_FROM_HISTORY, BROUGHT_TO_FRONT
    int flags = intent.getFlags();
    if ((flags & 0x00100000) != 0 || (flags & 0x00400000) != 0) {
        intentCleared();
        return;
    }

    String type = intent.getType();
    ArrayList<String> uriList = new ArrayList<>();
    ArrayList<String> textList = new ArrayList<>();

    // ClipData (SEND_MULTIPLE, SEND с URI)
    ClipData clipData = intent.getClipData();
    if (clipData != null) {
        for (int i = 0; i < clipData.getItemCount(); i++) {
            ClipData.Item item = clipData.getItemAt(i);
            if (item.getUri() != null) uriList.add(item.getUri().toString());
            if (item.getText() != null) textList.add(item.getText().toString());
        }
    }

    // ACTION_VIEW — getData()
    if (uriList.isEmpty() && Intent.ACTION_VIEW.equals(action)) {
        Uri uri = intent.getData();
        if (uri != null) uriList.add(uri.toString());
    }

    // ACTION_SEND — STREAM extra
    if (uriList.isEmpty() && (Intent.ACTION_SEND.equals(action) || Intent.ACTION_SEND_MULTIPLE.equals(action))) {
        if (Intent.ACTION_SEND_MULTIPLE.equals(action)) {
            ArrayList<Uri> uris = intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM);
            if (uris != null) for (Uri u : uris) uriList.add(u.toString());
        } else {
            Uri stream = intent.getParcelableExtra(Intent.EXTRA_STREAM);
            if (stream != null) uriList.add(stream.toString());
        }
    }

    // ACTION_SEND text/plain — EXTRA_TEXT
    if (textList.isEmpty() && Intent.ACTION_SEND.equals(action) && "text/plain".equals(type)) {
        String text = intent.getStringExtra(Intent.EXTRA_TEXT);
        if (text != null) textList.add(text);
    }

    String[] urisArr = uriList.toArray(new String[0]);
    String[] textsArr = textList.toArray(new String[0]);

    intentReceived(action, type != null ? type : "", urisArr, textsArr);
}
```

