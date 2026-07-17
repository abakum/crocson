# Plan: тап по видоискателю QR → вкл/выкл вспышки (torch)

## Контекст
QR-сканер показывает превью в нативном `Dialog` (`GoNativeActivity.java`): корневой
`FrameLayout` содержит `SurfaceView` (камера) снизу и `QrOverlayView` (рамка-видоискатель)
сверху, на весь диалог (`MATCH_PARENT`). Камера — Camera1. Сейчас вспышка нигде не
настраивается, тач-обработки на диалоге/оверлее нет, отменa — только Back/Cancel
(`setCancelable(true)`).

**Важно (поправка к ранней оценке):** фича целиком укладывается в Java. Мост cgo/JNI
(`for_android.*`, `qr_camera.go`) **не трогается** — вспышка это UI-тап + Camera1-операция,
Go в ней не участвует, QR-декод не меняется.

## Решения (зафиксированы)
1. **Только Java.** Никаких правок Go/cgo/`for_android.*`, новых JNI-методов нет.
2. **Взаимодействие:** тап в любом месте видоискателя переключает вспышку (буквально
   по запросу пользователя). Отдельная иконка-индикатор **не добавляется** — физический
   LED сам по себе обратная связь (вопрос об иконке отклонён/отложен пользователем).
3. **Режим:** только `FLASH_MODE_TORCH` / `FLASH_MODE_OFF` (постоянный фонарик), не
   `AUTO`/одиночная вспышка.
4. **Capability-guard:** если задняя камера не сообщает `FLASH_MODE_TORCH` в
   `getSupportedFlashModes()` — переключатель молча no-op (тост не добавляем, torch
   доступен на подавляющем большинстве устройств).
5. **Состояние «одна сессия»:** `qrFlashOn` сбрасывается в `false` при остановке камеры
   (`stopCamera()`). Новое сканирование всегда стартует с выключенной вспышкой.
6. **Честность в окне до открытия камеры:** тап во время краткого окна
   «диалог показан, камера ещё не открыта» всё равно инвертирует `qrFlashOn`, а
   `startCameraWithHolder` применит текущее значение сразу после `startPreview()`.

## Явно вне scope (НЕ трогать)
- Иконка/кнопка вспышки с визуальным индикатором состояния.
- Camera2/CameraX (сборка только против `android.jar`, см. комментарий у камеры).
- Сохранение состояния вспышки между сессиями сканирования.
- Изменения в Go/cgo/JNI (`for_android.*`, `qr_camera.go`).
- Режимы `FLASH_MODE_AUTO`/`FLASH_MODE_ON`.

## Реализация (`GoNativeActivity.java`, единственный правимый файл)

### A. Новые поля (рядом с `qrCamera`, ≈265–266)
```java
// Состояние фонарика QR-камеры. Инвертируется тапом по оверлею превью;
// применяется на камерном потоке как FLASH_MODE_TORCH/OFF. Сбрасывается в OFF
// при остановке камеры (один переключатель на живую сессию).
private static volatile boolean qrFlashOn = false;
// Задняя камера поддерживает FLASH_MODE_TORCH (запрос一次 в startCameraWithHolder).
// Гарант для toggleCameraFlash (молчаливый no-op если false).
private static volatile boolean qrFlashTorchSupported = false;
```
Оба `volatile`: `qrFlashOn` пишется на UI-потоке (клик), читается на камерном;
`qrFlashTorchSupported` пишется на камерном, читается на UI.

### B. Обнаружение поддержки (в `startCameraWithHolder`, блок `params`, рядом с
выбором focus-режима ≈712–717)
```java
try {
    List<String> flashModes = params.getSupportedFlashModes();
    qrFlashTorchSupported = flashModes != null
            && flashModes.contains(Camera.Parameters.FLASH_MODE_TORCH);
} catch (Throwable ignored) { qrFlashTorchSupported = false; }
```

### C. Применить состояние сразу после открытия (в `startCameraWithHolder`, после
`c.startPreview();` ≈800, до `return true;`)
```java
// Уважать последний тап пользователя (напр. тап в окне до открытия камеры).
applyCameraFlash(c);
```

### D. Новые методы (рядом с `reapplyPreviewOrientation()`, ≈826)
```java
// Переключить фонарик. Вызывается из click-listener'а оверлея (UI-поток).
// Инвертирует желаемое состояние и применяет на камерном потоке (все camera-операции
// только на qrCameraHandler). Молчаливый no-op, если задняя камера без FLASH_MODE_TORCH.
private static void toggleCameraFlash() {
    if (!qrFlashTorchSupported) return;
    qrFlashOn = !qrFlashOn;
    final boolean on = qrFlashOn;
    ensureCameraThread();
    qrCameraHandler.post(new Runnable() {
        @Override
        public void run() {
            Camera c = qrCamera;
            if (c != null) applyCameraFlash(c, on);
        }
    });
}

// Применить текущее qrFlashOn к открытой камере. Камерный поток.
private static void applyCameraFlash(Camera c) { applyCameraFlash(c, qrFlashOn); }

private static void applyCameraFlash(Camera c, boolean on) {
    try {
        Camera.Parameters params = c.getParameters();
        params.setFlashMode(on ? Camera.Parameters.FLASH_MODE_TORCH
                               : Camera.Parameters.FLASH_MODE_OFF);
        c.setParameters(params);
    } catch (Throwable t) {
        Log.e(TAG, "Java: setFlashMode failed: " + t.getMessage());
    }
}
```

### E. Click-listener на оверлей (в `showCameraDialog`, после `root.addView(overlayView);` ≈643)
```java
overlayView.setOnClickListener(new View.OnClickListener() {
    @Override
    public void onClick(View v) { toggleCameraFlash(); }
});
```
`setOnClickListener` делает `QrOverlayView` кликабельным и поглощает тап внутри
диалога. `MotionEvent`-импорт не нужен. Импорты `android.hardware.Camera` и
`android.view.View` уже присутствуют (строки 16, 36).

### F. Сброс состояния (в `stopCamera()`, рядом с очисткой статических полей ≈917–920)
```java
qrFlashOn = false;
qrFlashTorchSupported = false;
```

## Риски и поведение
- **Потокобезопасность:** все camera-операции (`getParameters`/`setFlashMode`/
  `setParameters`) выполняются только на `qrCameraHandler` — внутри `toggleCameraFlash`
  (через `post`) и в `startCameraWithHolder`/`applyCameraFlash` (уже на камерном потоке).
  Чтение `qrCamera` — на камерном потоке внутри Runnable (не захватывается «свежим» с UI).
- **Смена flash в live-preview:** стандартный Camera1-паттерн `getParameters`→
  `setFlashMode`→`setParameters` работает «на лету» на подавляющем большинстве устройств.
  Отдельные старые HAL'ы требуют `stopPreview`/`startPreview` вокруг смены параметров —
  если на целевом устройстве torch не зажигается, добавить stop/start вокруг
  `setParameters` (известный Camera1-квирк, при необходимости — отдельная правка).
- **Тап vs отмена:** тап **внутри** диалога никогда его не отменял (отмена — только Back/
  Cancel через `setCancelable(true)`, плюс cancel-on-touch-outside срабатывает лишь по
  тапу **за пределами** окна диалога). Сделать оверлей кликабельным — конфликта с отменой нет.
- **Без torch:** на устройствах без `FLASH_MODE_TORCH` тап молча nothing (состояние даже
  не инвертируется), декод QR не страдает.
- **Быстрый double-tap:** каждый тап инвертирует флаг и пере-применяет режим — лишняя
  работа, но безвредно; дебаунс не нужен.

## Валидация
1. **Компиляция:** `make arm64 wsl` (обе платформы должны собраться; ошибка Java-компиляции
   всплывёт на `make arm64`, который компилирует `.java` в APK).
2. **Рантайм (вручную, устройство с задней вспышкой):**
   - Открыть QR-сканер → тап по видоискателю зажигает torch, повторный тап — гасит.
   - Расшифровка QR по-прежнему работает (кадры идут в Go, gozxing декодирует) и при
     включённом, и при выключенном фонарике.
   - Отмена (Back/Cancel) и `onPause` корректно закрывают камеру; повторное открытие
     сканера стартует с **выключенной** вспышкой.
   - Поведение на устройстве **без** torch: тап ничего не ломает (no-op), декод работает.
3. Проверить, что `qr_camera.go` / `for_android.*` остались нетронутыми (`git diff --stat`).
