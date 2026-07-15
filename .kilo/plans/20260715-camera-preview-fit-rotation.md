# Plan: выбор разрешения превью по обоим ориентациям (без переполнения после поворота)

## Контекст / баг
Разрешение превью фиксируется при открытии камеры. Текущий `choosePreviewSize` ограничивает
его по usable **текущей** ориентации. В портрете малая сторона = полный short edge
(бары на длинном ребре) → напр. 1080 вписывается. После поворота в ландшафт та же 1080
становится высотой диалога, а usable-высота ландшафта = `shortEdge − бары` = 960
(бары уже на коротком ребре) → переполнение. `updateDialogSizeToCameraResolution` лишь
свопает ориентацию, фиксированное разрешение он не уменьшает. Асимметрично:
ландшафт→портрет вписывается, портрет→ландшафт — нет.

Рецепт пользователя: «на портрете ширина диалога не должна превышать высоту ландшафта без
статус-бара и наоборот» — т.е. ограничивать малую сторону превью по наихудшей (по обеим
ориентациям) usable малой стороне.

## Решение
Выбирать превью по **ориентационно-независимым** границам от физических рёбер экрана, а не от
текущей ориентации. Тогда выбранное разрешение вписывается в обе ориентации и не переполняется
при повороте.

Границы (UI-поток):
```
realW, realH = Display.getRealSize()            // полный физический размер
sum = insets.top + insets.bottom + insets.left + insets.right   // Σ баров
shortEdge = min(realW, realH)
longEdge  = max(realW, realH)
smallBound = shortEdge - sum        // ограничивает МАЛУЮ сторону превью (ключ)
largeBound = longEdge  - sum        // ограничивает большую сторону (щедро/безопасно)
```
`smallBound` — это и есть «ширина портрета ≤ высота ландшафта без статус-бара».

**Почему безопасно:** `Σinsets ≥` баров на коротком ребре в любой ориентации, поэтому
`shortEdge − Σinsets ≤` реальной малой usable в любой ориентации. Консервативно на части
устройств (даёт 880 там, где реально можно 960 — точное расщепление баров другой ориентации
из текущей не читается), но переполнения не даёт никогда.

## Реализация — `GoNativeActivity.java`

### 1. Новый хелпер `getPreviewBounds()` (UI-поток, рядом с `getUsableScreenSize`)
Возвращает `{largeBound, smallBound}` по формулам выше (getRealSize + Σinsets).

### 2. `choosePreviewSize(sizes, largeBound, smallBound)` (≈456)
Наибольшая площадь при `max(s.w,s.h) ≤ largeBound && min(s.w,s.h) ≤ smallBound`;
фолбэк (ничто не вписалось) — наименьшая площадь; `null` если список пуст.
(Предикат ориентации больше не нужен — границы от физических рёбер.)

### 3. `getCameraPreviewSize(largeBound, smallBound)` (≈379)
Сменить сигнатуру; внутренний вызов `choosePreviewSize(sizes, largeBound, smallBound)`.

### 4. Кэш `qrUsableW/H` → пара `qrBoundLarge`/`qrBoundSmall` (volatile, ≈306)
Ставятся на UI-потоке в `showCameraDialog`, читаются на камерном в `startCameraWithHolder`,
сбрасываются в `stopCamera`.

### 5. `showCameraDialog()` (≈562–575)
```java
int[] b = getPreviewBounds();
int largeBound = b[0], smallBound = b[1];
qrBoundLarge = largeBound; qrBoundSmall = smallBound;
Camera.Size previewSize = getCameraPreviewSize(largeBound, smallBound);
```
`getUsableScreenSize()` оставить для `computeDialogSize` (определение портрет/ландшафт) и лога.

### 6. `startCameraWithHolder()` (≈696, камерный поток)
```java
int lg = qrBoundLarge > 0 ? qrBoundLarge : 640;
int sm = qrBoundSmall > 0 ? qrBoundSmall : 480;
Camera.Size chosen = choosePreviewSize(sizes, lg, sm);
```

### 7. `stopCamera()` (≈912)
`qrBoundLarge = 0; qrBoundSmall = 0;`

### Без изменений
`computeDialogSize`, `updateDialogSizeToCameraResolution` — они переориентируют уже
вписывающееся разрешение. `getUsableScreenSize()` сохраняется (определение ориентации).

## Валидация
1. **Компиляция:** `make arm64 wsl` (Java-ошибка всплывёт на `make arm64`).
2. **Рантайм (устройство, видимые статус-бар/навигация):**
   - Старт в **портрете** → поворот в ландшафт: диалог не выходит под статус-бар, без искажений.
   - Старт в **ландшафте** → поворот в портрет: то же.
   - Сравнить лог `camera preview resolution=…`: в портрете малая сторона ≤ smallBound
     (напр. ≤880/960, а не 1080); декод QR работает в обеих ориентациях.
   - Поворот во время сканирования пересчитывается (`updateDialogSizeToCameraResolution` +
     `reapplyPreviewOrientation`).
