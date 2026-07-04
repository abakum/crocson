# Изменение типа Foreground Service с dataSync на mediaPlayback

## Проблема

Huawei AppGallery исключил Индию из списка стран публикации, вероятно, из-за ассоциации `dataSync` с VPN/Proxy-приложениями. Текущий тип `FOREGROUND_SERVICE_TYPE_DATA_SYNC` может быть ошибочно интерпретирован как VPN-сервис.

## Решение

Изменить тип foreground service с `dataSync` на `mediaPlayback`, так как WebDAV сервер часто используется для передачи медиа файлов, и этот тип не ассоциируется с VPN/Proxy.

## Изменения

### 1. AndroidManifest.xml

**Строка 67 — Изменить тип сервиса:**
```xml
<!-- Было: -->
<service
    android:name=".CrocsonService"
    android:foregroundServiceType="dataSync"
    android:exported="false" />

<!-- Станет: -->
<service
    android:name=".CrocsonService"
    android:foregroundServiceType="mediaPlayback"
    android:exported="false" />
```

**Строка 111 — Изменить разрешение:**
```xml
<!-- Было: -->
<uses-permission android:name="android.permission.FOREGROUND_SERVICE_DATA_SYNC" />

<!-- Станет: -->
<uses-permission android:name="android.permission.FOREGROUND_SERVICE_MEDIA_PLAYBACK" />
```

### 2. CrocsonService.java

**Строка 63 — Изменить тип в startForeground():**
```java
// Было:
if (Build.VERSION.SDK_INT >= 34) {
    startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
} else {
    startForeground(NOTIFICATION_ID, notification);
}

// Станет:
if (Build.VERSION.SDK_INT >= 34) {
    startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK);
} else {
    startForeground(NOTIFICATION_ID, notification);
}
```

## Почему оба файла должны быть изменены?

На Android 14+ (API 34+) метод `startForeground()` принимает третий параметр — тип сервиса. Этот тип **ДОЛЖЕН** соответствовать тому, что указано в `AndroidManifest.xml`.

Если типы не совпадают, приложение упадет с ошибкой:
```
SecurityException: Service type does not match the type specified in manifest
```

## Проверка

После изменений:

1. Убедитесь, что нет упоминаний `dataSync` в коде:
   ```bash
   grep -r "DATA_SYNC" --include="*.xml" --include="*.java" .
   ```

2. Соберите APK и проверьте, что:
   - AndroidManifest.xml содержит `mediaPlayback`
   - CrocsonService.java содержит `FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK`

3. Протестируйте на Android 14+ устройстве:
   - Запустите WebDAV сервер
   - Убедитесь, что foreground service работает и не убивается в фоне
   - Проверьте, что уведомление отображается корректно

## Дополнительные улучшения (опционально)

Можно обновить текст уведомления, чтобы он более точно отражал функциональность:

**CrocsonService.java строки 46-47 и 54-55:**
```java
.setContentTitle("crocson")
.setContentText("WebDAV server running")  // можно оставить или уточнить, например "File transfer active"
```

Это не обязательно, но может помочь с модерацией.