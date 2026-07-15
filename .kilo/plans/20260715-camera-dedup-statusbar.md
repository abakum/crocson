# Plan: дедупликация камерного блока GoNativeActivity.java + учёт статус-бара + AGENTS.md

## Контекст
После реализации `qr_camera.go` в камерном блоке `GoNativeActivity.java` накопилось дублирование:
- блок получения размера экрана через `Display.getSize()` повторяется **4 раза**;
- цикл выбора разрешения превью («наибольшее превью, чья макс. сторона ≤ экран») дублируется **2 раза**;
- расчёт размера диалога со swap портрет/ландшафт дублируется **2 раза**.

Заодно вскрывается баг: оба места расчёта размера диалога берут «голый» размер экрана и **не вычитают системные inset'ы**. Тема активности — `Theme.DeviceDefault…NoActionBar` (НЕ fullscreen), `statusBarColor`/`windowLightStatusBar` заданы явно → статус-бар видим, окно приложения под ним. Поэтому разрешение превью выбирается по полному экрану, и `dialogH` (в портрете ≈ screenH) оказывается больше usable-высоты → центрированный через `Gravity.CENTER` диалог залезает под статус-бар. Приложение уже умеет читать inset'ы (`updateLayout()` → `WindowInsets.getSystemWindowInsetTop/Bottom/...` → `insetsChanged()`), просто камерный блок их не использует.

## Решения (зафиксированы)
1. Дедуп: режим «только хелперы», поведение сохраняется **везде, кроме одного целевого изменения**.
2. Целевое изменение поведения: выбор разрешения превью и размер диалога считаются относительно usable-области (экран минус статус-бар и навигация через `WindowInsets`). Диалог вписывается под статус-бар.
3. `AGENTS.md` (отсутствует): создать, зафиксировать команду проверки `make arm64 wsl`.

## Явно вне scope (НЕ трогать в этой задаче)
- Синхронный `Camera.open()` в `getCameraPreviewSize` на UI-потоке (существующий риск ANR) — сохранить как есть.
- Дублирование cursor-запросов вне камерного блока (`getFileName`, `getMimeType`, `getSize`, `getModTime`, …) — не связано с qr-работой.
- Режим «draw behind» статус-бара (диалог на весь физический экран) — отклонён пользователем.

## Реализация

### A. `GoNativeActivity.java` — новые хелперы (в камерном блоке, рядом с `getDeviceRotation()`)

```java
// UI-thread ONLY. Screen size minus current system insets (status bar / nav bar).
// Falls back to raw getSize() when insets are unavailable (same as today).
private static int[] getUsableScreenSize() {
    int screenW = 640, screenH = 480;
    try {
        android.graphics.Point p = new android.graphics.Point();
        goNativeActivity.getWindowManager().getDefaultDisplay().getSize(p);
        screenW = p.x; screenH = p.y;
        WindowInsets insets = goNativeActivity.getWindow().getDecorView().getRootWindowInsets();
        if (insets != null) {
            screenW -= (insets.getSystemWindowInsetLeft() + insets.getSystemWindowInsetRight());
            screenH -= (insets.getSystemWindowInsetTop() + insets.getSystemWindowInsetBottom());
        }
    } catch (Throwable ignored) {}
    return new int[]{Math.max(1, screenW), Math.max(1, screenH)};
}

// Largest preview whose max side <= maxSide (ties -> biggest area); else first.
private static Camera.Size choosePreviewSize(List<Camera.Size> sizes, int maxSide) {
    Camera.Size chosen = null;
    if (sizes != null && !sizes.isEmpty()) {
        for (Camera.Size s : sizes) {
            if (Math.max(s.width, s.height) <= maxSide) {
                if (chosen == null || (s.width * s.height) > (chosen.width * chosen.height)) {
                    chosen = s;
                }
            }
        }
        if (chosen == null) chosen = sizes.get(0);
    }
    return chosen;
}

// Dialog W/H for the camera resolution, swapping for portrait.
private static int[] computeDialogSize(int screenW, int screenH, int camW, int camH) {
    if (screenW > screenH) return new int[]{camW, camH}; // landscape
    return new int[]{camH, camW};                         // portrait
}
```

### B. Связующее состояние для камерного потока
```java
// usable max side captured on the UI thread in showCameraDialog, read by
// startCameraWithHolder on the camera thread (removes off-thread Display access).
private static volatile int qrUsableMaxSide = 0;
```

### C. Сводка точек правки (заменить инлайн-дубли на вызовы хелперов)
1. `getCameraPreviewSize()` (≈374) → сменить сигнатуру на `getCameraPreviewSize(int maxSide)`: убрать внутренний `getSize()`-блок, внутренний цикл выбора заменить на `choosePreviewSize(sizes, maxSide)`. `Camera.open()`/`release()` оставить (вне scope).
2. `showCameraDialog()` (≈539): заменить `getSize()`-блок на `int[] u = getUsableScreenSize(); int usableW=u[0], usableH=u[1]; int usableMax=Math.max(usableW,usableH); qrUsableMaxSide=usableMax;`. `getCameraPreviewSize(usableMax)`. Заменить swap-блок (≈555) на `computeDialogSize(usableW, usableH, cameraW, cameraH)`.
3. `startCameraWithHolder()` (≈670, камерный поток): убрать внутренний `getSize()`-блок (≈675) и цикл выбора (≈682). Использовать `int usableMax = qrUsableMaxSide > 0 ? qrUsableMaxSide : Math.max(640,480);` и `Camera.Size chosen = choosePreviewSize(sizes, usableMax);`. Выбор фокус/FPS оставить без изменений.
4. `updateDialogSizeToCameraResolution()` (≈826, тело в `runOnUiThread`): заменить `getSize()`-блок (≈834) на `getUsableScreenSize()`; заменить swap-блок (≈839) на `computeDialogSize(usableW, usableH, cameraW, cameraH)`.
5. Сбрасывать `qrUsableMaxSide = 0` в `stopCamera()` (≈907) рядом с очисткой других статических полей, чтобы не тащить устаревшее значение между сессиями сканирования.

### D. `AGENTS.md` (новый файл в корне репозитория)
Минимальный, ориентированный на сборку/проверку. Содержание:
- Краткое описание: crocson — Go + Fyne, цели Android и Windows.
- **Проверка кода:** запускать `make arm64 wsl` перед завершением работы.
  - `make arm64` → `fyne package -os android/arm64 --release --sign`: компилирует Go **и** `GoNativeActivity.java` (плюс прочие `.java`) в APK.
  - `make wsl` → `GOOS=windows CC=x86_64-w64-mingw32-gcc ... go build`: Windows-бинарник через mingw в WSL.
  - Иными словами, `make arm64 wsl` покрывает компиляцию на обеих целевых платформах.
- Build tags: android — основная цель; для gopls включать через `make atags` (`-tags=android`); сброс — `make tags`.
- Мост Java↔Go: `GoNativeActivity.java` (Activity + камерный блок), `for_android.go`/`for_android.c` (cgo), `qr_camera.go` (QR-декодер на стороне Go).

## Риски и поведение
- **Единственное поведение-изменение:** источник `maxSide`/`screenW/H` для камеры сменён с «голого экрана» на usable-область. Разрешение превью может стать чуть меньше, чем раньше (чтобы диалог гарантированно влезал под статус-бар). Это и есть цель фикса.
- Фоллбэк при `getRootWindowInsets() == null` (ранний жизненный цикл) → raw `getSize()` → эквивалент сегодняшнего поведения. Поведение сохраняется, когда inset'ы недоступны.
- Все чтения `WindowInsets` — только на UI-потоке (`showCameraDialog`/`updateDialogSizeToCameraResolution` уже в `runOnUiThread`; `onConfigurationChanged` идёт в main). Камерный поток inset'ы не трогает — использует кэш `qrUsableMaxSide`.
- `minSdkVersion=23` → `getSystemWindowInset*` доступны; defensive `try/catch` достаточен.

## Валидация
1. **Компиляция:** `make arm64 wsl` (обе платформы должны собраться; ошибка компиляции `GoNativeActivity.java` всплывёт на этапе `make arm64`).
2. **Рантайм (вручную, устройство с видимыми статус-баром и навигацией):**
   - Портрет: открыть QR-сканер → камерный диалог не залезает под статус-бар, превью центрировано ниже него, рамка-оверлей отрисована корректно.
   - Ландшафт: то же самое, диалог вписан в usable-область.
   - Декод QR по-прежнему работает (кадры идут в Go, gozxing декодирует).
   - Поворот во время сканирования: `onConfigurationChanged` → `updateDialogSizeToCameraResolution` пересчитывает размер с учётом inset'ов; `reapplyPreviewOrientation` работает.
   - Отмена (Back/Cancel) и `onPause` корректно закрывают камеру.
3. Проверить, что не осталось закомментированных/мёртвых копий старого `getSize()`-блока в камерном блоке.
