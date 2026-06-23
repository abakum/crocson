# План: тема Activity и совместимость манифеста с Huawei

## Контекст
- Текущая тема Activity: `@android:style/Theme.Light` (AndroidManifest.xml:19).
- Дефолт fyne (templates/data/AndroidManifest.xml:12): `@android:style/Theme`.
- Fyne рендерит собственный GL-поверхностью (OpenGL ES), тема влияет **только** на фон окна до первого кадра и на системные бары — НЕ на UI приложения (тёмная/светлая у crocson своя, через `setDarkMode` в Go).
- Тема **не была причиной краша** (причина — 4KB-align, уже починено). Меняем ради совместимости/Huawei-харденинга.

## Рекомендация по теме

**Предлагаемая:** `@android:style/Theme.DeviceDefault.NoActionBar`
```xml
android:theme="@android:style/Theme.DeviceDefault.NoActionBar"
```
Обоснование:
- **Фреймворк-тема** (не требует AppCompat, который crocson не бандлит) — доступна с API 21, у нас minSdk 23.
- **`DeviceDefault`** подстраивается под тему устройства (EMUI/HarmonyOS) → нет белой вспышки на тёмных темах и нет конфликта с рескином Huawei.
- **`NoActionBar`** — GL-поверхность Fyne заполняет контент целиком, лишний action bar не нужен и на EMUI мог бы накладываться.
- `Theme.Light` устаревшая и даёт белую вспышку при тёмной теме устройства.

**Альтернативы (если `DeviceDefault` не устроит):**
- `@android:style/Theme.NoTitleBar` — максимально консервативно, без декораций, поведение как у дефолта fyne.
- `@android:style/Theme.DeviceDefault.Light.NoActionBar` — если нужен именно светлый фон окна.

## Улучшения совместимости с Huawei

### 1. (главное) Добавить Huawei Browser в `<queries>` — AndroidManifest.xml:97-104
Сейчас блок `<queries>` перечисляет `com.android.chrome`, `mark.via.gp`, `com.sec.android.app.sbrowser`, `com.opera.mini.native`, `com.microsoft.emmx`, `org.mozilla.firefox`. На Huawei-телефонах дефолтный браузер — **Huawei Browser (`com.huawei.browser`)**. На Android 11+ (targetSdk 36) без записи в `<queries>` он **невидим** для `queryIntentActivities`/`resolveActivity`.
crocson использует этот список браузеров как фолбэк для «Scan QRs» (applinks.go:464-469) — на Huawei фолбэк не сработает.

Добавить строку:
```xml
<package android:name="com.huawei.browser" />
```

### 2. (опционально, зеркало в Go) Добавить Huawei Browser в список сканера — applinks.go:464-469
Чтобы «Scan QRs» реально пробовал Huawei Browser, добавить туда же:
```go
{Package: "com.huawei.browser", Categories: []string{CATEGORY_BROWSABLE}},
```
Без п.1 это бессмысленно (пакет невидим), поэтому делается вместе.

### 3. (опционально, минор) configChanges — AndroidManifest.xml:16
Добавить `smallestScreenSize|density`, чтобы складные/большие экраны Huawei и смена плотности не пересоздавали Activity:
```xml
android:configChanges="keyboard|keyboardHidden|orientation|screenSize|screenLayout|uiMode|smallestScreenSize|density"
```

### 4. (опционально, гигиена) Storage-разрешения — AndroidManifest.xml:107-108
При targetSdk 36 `WRITE/READ_EXTERNAL_STORAGE` фактически no-op (scoped storage). Можно ограничить:
```xml
<uses-permission android:name="android.permission.WRITE_EXTERNAL_STORAGE" android:maxSdkVersion="28" />
<uses-permission android:name="android.permission.READ_EXTERNAL_STORAGE" android:maxSdkVersion="29" />
```
На краш/работу не влияет; не делать, если есть legacy-код, опирающийся на эти пермишены до API 29.

## Подтверждённый объём (выбор пользователя)
- **Тема:** `@android:style/Theme.DeviceDefault.NoActionBar`
- **Huawei Browser в `<queries>`:** да
- **Storage-пермишены:** ограничить `maxSdkVersion`
- НЕ делаем: Go-зеркало браузера, configChanges для складных

## Конкретные правки

### A. Тема — AndroidManifest.xml:19
```xml
android:theme="@android:style/Theme.DeviceDefault.NoActionBar"
```
(было `@android:style/Theme.Light`)

### B. Huawei Browser в `<queries>` — AndroidManifest.xml (после строки org.mozilla.firefox)
```xml
<package android:name="com.huawei.browser" />
```

### C. Storage-пермишены — AndroidManifest.xml:107-108
```xml
<uses-permission android:name="android.permission.WRITE_EXTERNAL_STORAGE" android:maxSdkVersion="28" />
<uses-permission android:name="android.permission.READ_EXTERNAL_STORAGE" android:maxSdkVersion="32" />
```
Значения:
- `WRITE_EXTERNAL_STORAGE` `maxSdkVersion=28` — начиная с API 29 (scoped storage) он no-op, нужен только до 28.
- `READ_EXTERNAL_STORAGE` `maxSdkVersion=32` — действует до API 32; с API 33 устарел в пользу гранулярных медиа-пермов.

**Риск/нюанс:** в `GoNativeActivity.sendIntentURIs` для `file://` URI код вызывает `checkSelfPermission("READ_EXTERNAL_STORAGE")` и `requestPermissions(...)` на всех API. После cap на API 33+ этот пермишен не объявлен → `checkSelfPermission` вернёт "не выдан" → запрос проигнорируется (пермишена нет) → `file://`-URI на API 33+ не пройдут. Но на API 33+ внешние `file://` практически не встречаются (приложения отдают `content://`), поэтому допустимо. Если проявится — вернуть READ без cap.

## Порядок выполнения
1. Правки A, B, C в AndroidManifest.xml.
2. `make arm64` → `aapt dump badging crocson.apk` убедиться, что тема применилась и `com.huawei.browser` не ломает сборку.
3. Прогнать на Huawei (Cloud Testing / устройство), проверить «Scan QRs» и запуск.

## Риски / открытые вопросы
- Ни одна правка не влияет на краш (причина — 4KB-align, починен отдельно).
- READ_EXTERNAL_STORAGE cap может заблокировать редкие `file://` на API 33+ — контролируемо, откатывается снятием cap.
