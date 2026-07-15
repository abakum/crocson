# Plan: убрать синхронное открытие камеры на UI-потоке

## Контекст / проблема
`getCameraPreviewSize()` (`GoNativeActivity.java:381`) — единственный синхронный
`Camera.open()` на UI-потоке. Он вызывается из `showCameraDialog` (тело в `runOnUiThread`),
только чтобы заранее узнать поддерживаемые размеры превью и размерить диалог. `Camera.open` —
блокирующая аппаратная операция → риск ANR/фриза UI. При этом `startCameraWithHolder`
(камерный поток) всё равно открывает камеру заново и сам выбирает реальный размер через
`choosePreviewSize`, а `updateDialogSizeToCameraResolution` подгоняет диалог. То есть open №2
делает open №1 (и его блокировку UI) избыточным.

## Error-пути (проверено по коду — НЕ меняются)
Отсутствие/ошибку камеры детектит `startCameraWithHolder` на камерном потоке, а не
`getCameraPreviewSize`:
- `findBackCameraId() < 0` → `failCameraOpen()` (`:710`).
- `Camera.open(id)` бросил → catch → `failCameraOpen()` (`:825`).
- `failCameraOpen()` (`:834`): `dismissCameraDialog()` + `lifecycleEvent("cameraOpenFailed")`
  → Go `qrCameraOpenFailed()` → `showToast("Camera unavailable")` + `qrFinish()`.

Сегодня при `getCameraPreviewSize == null` диалог уже создаётся на **640×480** и затем
закрывается `failCameraOpen`. После удаления — идентично (старт 640×480 → `failCameraOpen` →
тост). Только success-path меняется: вместо старта на реальном разрешении — старт 640×480 и
подгон `updateDialogSizeToCameraResolution` после открытия камеры.

## Решение (зафиксировано)
1. Удалить `getCameraPreviewSize` целиком.
2. В `showCameraDialog` стартовать диалог на фиксированном дефолте **640×480** (без запроса
   камеры). Реальный размер ставит камерный поток.

Размер-заглушка: 640×480 (выбор пользователя). Границы/кэш (`getPreviewBounds`,
`qrBoundLarge/Small`) не трогать — они нужны `startCameraWithHolder`.

## Реализация — `GoNativeActivity.java`

### 1. `showCameraDialog()` (≈604–609)
Заменить:
```java
Camera.Size previewSize = getCameraPreviewSize(largeBound, smallBound);
int cameraW = 640, cameraH = 480;
if (previewSize != null) {
    cameraW = previewSize.width;
    cameraH = previewSize.height;
}
```
на:
```java
int cameraW = 640, cameraH = 480;
```
(далее как прежде: `computeDialogSize(screenW, screenH, cameraW, cameraH)` → начальный
`dialogW/dialogH`; `largeBound`/`smallBound` уже установлены в `qrBoundLarge/Small` для
камерного потока).

### 2. Удалить метод `getCameraPreviewSize` (≈381–394)
После правки п.1 вызовов не остаётся (grep: единственный вызывающий — `showCameraDialog`).
`choosePreviewSize` остаётся — его использует `startCameraWithHolder`.

### Без изменений
`getPreviewBounds`, `qrBoundLarge/qrBoundSmall`, `choosePreviewSize`,
`startCameraWithHolder` (читает кэш → `choosePreviewSize` → `updateDialogSizeToCameraResolution`),
`failCameraOpen`, `computeDialogSize`, `updateDialogSizeToCameraResolution`, Go-сторона
(`qr_camera.go`).

## Риски / поведение
- **Success:** диалог кратко 640×480 (портрет → 480×640), затем `updateDialogSizeToCameraResolution`
  подгоняет под реальное разрешение (видимое «вырастание»). Без искажений (поверхность 1:1 к
  разрешению после подгона).
- **No camera / open error:** идентично сегодняшнему — вспышка 640×480 → `failCameraOpen` →
  закрытие + тост «Camera unavailable».
- Остался единственный `Camera.open` (в `startCameraWithHolder`, камерный поток); статические
  `getNumberOfCameras/getCameraInfo` (в `findBackCameraId`, `getCameraSensorOrientation`) — не
  блокирующие.
- Существующий граничный случай (не от этого изменения): если `surfaceCreated` не сработает,
  `failCameraOpen` не запустится — но normalmente он срабатывает.

## Валидация
1. **Компиляция:** `make arm64 wsl` (Java-ошибка всплывёт на `make arm64`).
2. **Рантайм (устройство):**
   - Открыть QR-скан: диалог кратко 640×480 → щёлкает к разрешению камеры, без искажений; QR
     декодируется.
   - Нет камеры / камера занята/отключена: вспышка диалога → закрытие → тост «Camera unavailable».
   - В логе: один `startCamera WxH` (один open), без отдельного open под размер диалога.
