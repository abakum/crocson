# Fix: EGL_BAD_SURFACE на Android 9

## Проблема

На Android 9 (API ≤ 28) EGL surface становится невалидным при возврате из фона.
`recreate()` НЕ помогает — EGL surface не успевает очиститься при пересоздании
в том же цикле. `finish()` работает, т.к. полностью уничтожает activity и даёт
EGL время на очистку.

## Фикс: `onUserLeaveHint()` + `finish()` для API ≤ 28

`onUserLeaveHint()` вызывается **только** при пользовательской навигации (Home,
Recents), но **НЕ** при открытии файлпикера, браузера или системных диалогов из
кода приложения.

### `GoNativeActivity.java` — два изменения

1. **Откатить** `recreate()` из `onRestart`:
```java
@Override
protected void onRestart() {
    super.onRestart();
    Log.d(TAG, "Java: onRestart");
    lifecycleEvent("restart");
}
```

2. **Добавить** `onUserLeaveHint` (после `updateTheme`, перед `onStart`):
```java
@Override
protected void onUserLeaveHint() {
    super.onUserLeaveHint();
    Log.d(TAG, "Java: onUserLeaveHint");
    if (Build.VERSION.SDK_INT <= 28) {
        finish();
    }
}
```

### Почему это работает

| Сценарий | onUserLeaveHint | finish() | Результат |
|---|---|---|---|
| Home / Recents | ✓ вызывается | ✓ activity уничтожена | onCreate → свежий EGL |
| Файлпикер | ✗ НЕ вызывается | ✗ не вызывается | onResume → EGL жив |
| Браузер из приложения | ✗ НЕ вызывается | ✗ не вызывается | onResume → EGL жив |
| Телефонный звонок | ✗ НЕ вызывается | ✗ не вызывается | onResume → EGL жив |
| Android 10+ | вызывается | ✗ API > 28, пропускаем | нормальный lifecycle |

### Риски

- При Home/Recents на Android 9: activity пересоздаётся (~500мс), `savedInstanceState`
  сохраняется, `LAUNCHED_FROM_HISTORY=true` → intent не репроцессится
- Мигание при пересоздании (как и с `recreate()`, но EGL гарантированно чистый)
