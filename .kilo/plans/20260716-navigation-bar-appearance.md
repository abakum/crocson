# Навигационная панель: синхронизация с принудительной темой (runtime)

## Контекст
- `res/values/themes.xml` и `res/values-night/themes.xml` уже правлены: добавлены
  `navigationBarColor` (white/black) и `windowLightNavigationBar` (true/false). Это
  **статичный baseline** по системному дню/ночи.
- `applyStatusBarIcons()` (`GoNativeActivity.java:972`) — это **runtime**-override, управляемый
  `appThemeMode` (system/light/dark из Go) + `systemDark`. Он перекрашивает только **status bar**
  и реагирует на *принудительно* выбранный режим темы.
- **Несоответствие:** при принудительной теме (light при тёмной системе или dark/black при светлой)
  статус-бар уходит в режим темы, а нав-бар остаётся на системном baseline → рассинхрон панелей.
- Решение: крутить нав-бар тем же методом, симметрично статус-бару. Бaseline в `themes.xml`
  оставить (он валиден до первого пуша режима из Go и no-op на edge-to-edge API 35+).

## API-факты (для имплементатора)
- `Window.setNavigationBarColor(int)` — API 21+; **no-op на edge-to-edge** (API 35+, targetSdk=36).
- `WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS` — API 30+ (работает и на edge-to-edge).
- `View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR` — API 26 (`Build.VERSION_CODES.O`).
- Все нужные импорты уже есть: `Build`, `View`, `Window`, `WindowInsetsController`, `Color`.
- minSdk=23, targetSdk=36 (`AndroidManifest.xml:117-118`).

## Изменения (единственный файл: `GoNativeActivity.java`)

### 1. Переименовать метод `972` и дополнить тело
`applyStatusBarIcons()` → `applySystemBarsAppearance()`. Новое тело:

```java
static void applySystemBarsAppearance() {
    final GoNativeActivity act = goNativeActivity;
    if (act == null) return;
    final boolean lightBg = (appThemeMode == 1) || (appThemeMode == 0 && !systemDark);
    act.runOnUiThread(new Runnable() {
        @Override
        public void run() {
            Window window = act.getWindow();
            if (window == null) return;
            try { window.setStatusBarColor(lightBg ? Color.WHITE : Color.BLACK); } catch (Throwable ignored) {}
            try { window.setNavigationBarColor(lightBg ? Color.WHITE : Color.BLACK); } catch (Throwable ignored) {}
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                WindowInsetsController c = window.getInsetsController();
                if (c != null) {
                    int mask = WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS
                             | WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS;
                    c.setSystemBarsAppearance(lightBg ? mask : 0, mask);
                }
            } else {
                View v = window.getDecorView();
                int f = v.getSystemUiVisibility();
                if (lightBg) f |= View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
                else f &= ~View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                    if (lightBg) f |= View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
                    else f &= ~View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
                }
                v.setSystemUiVisibility(f);
            }
        }
    });
}
```

Ключевые отличия от старого тела:
- добавлен `setNavigationBarColor(...)` в `try/catch` (симметрично статус-бару);
- в ветке API 30+ `mask` расширен обоими флагами, один вызов `setSystemBarsAppearance`;
- в ветке `else` добавлен `SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR` под guard `VERSION_CODES.O`.

### 2. Обновить callsite'ы (rename)
- `setAppThemeMode`, `GoNativeActivity.java:969` → `applySystemBarsAppearance();`
- `updateTheme`, `GoNativeActivity.java:1718` → `applySystemBarsAppearance();`

Других ссылок на `applyStatusBarIcons` в коде нет (3 хита: def + 2 callsite'а).

## Файлы, которые НЕ меняются
- `res/values/themes.xml`, `res/values-night/themes.xml` — уже правлены, остаются baseline.
- `for_android.go/.c/.h`, `theme.go`, `AndroidManifest.xml` — без изменений (режим уже пушится в Java).

## Валидация
1. `make arm64 wsl` — собирается APK (arm64) и Windows-бинарь (mingw). Java-ошибка всплывёт на
   `make arm64` (компиляция `.java` в `crocson.apk`).
2. На устройстве: для каждой темы (system/light/grey/dark/black) и системного режима (день/ночь) —
   **обе** панели (status + nav) совпадают по режиму (тёмные иконки на светлом фоне / светлые на
   тёмном). Особо проверить *принудительные* режимы против системной ночи/дня (раньше nav-bar
   отставал).
3. Переключение темы в рантайме → обе панели обновляются немедленно.
4. Регресс: проверить API 29/30+ (appearance-флаги) и API 26-29 (legacy `SYSTEM_UI_FLAG_*`); на
   edge-to-edge (API 35+) цвет игнорируется по дизайну, иконки остаются корректными.
5. Тень де-прекейшн-нотис от javac по `SYSTEM_UI_FLAG_*` — ожидаем (как и раньше).

## Риски / заметки
- На edge-to-edge (API 35+) `setNavigationBarColor` no-op по дизайну; appearance-флаги работают —
  аналогично статус-бару (см. план `202600714-status-bar-icon-contrast.md`).
- `appThemeMode` по умолчанию 0 (system) до пуша из Go → на старте нав-бар = baseline из
  `themes.xml`, что совпадает с `systemDark=false`/`true`. Рассинхрон только между реальным
  системным режимом и `systemDark`, но `onCreate:1567` сразу зовёт `updateTheme` → синхронно.
- Функция internal/static; rename затрагивает только этот файл.
