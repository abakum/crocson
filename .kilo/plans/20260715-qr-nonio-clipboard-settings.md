# Plan: non-IO QR-скан → буфер обмена + вкладка QR настроек

## Контекст / проблема
Сейчас декодированный QR-текст маршрутизируется в `qrRoute` (`qr_camera.go:166`):
- `IO`-префикс → `uriFromIntent` (заполняет секрет/relay-конфиг через `send.go:1676`/`fromURI`).
- иначе → `textFromIntent` → потребитель `send.go:1643` создаёт **файл для отправки** («корзина на передачу»).

Пользователь хочет: non-IO результат QR-скана **не** должен падать в корзину передачи, а должен
кладись в буфер обмена и открывать вкладку QR в `settings.go`, где под QR ссылку можно кликнуть
(перейти), а не-ссылку — просто прочитать.

## Различение QR vs Share (решено, без нового канала)
`textFromIntent` питают два разных поставщика:
- **Share-диалог** (Android intent): `intentTextNotify` (`for_android.go:1106`) — ДОЛЖЕН оставаться
  в корзине передачи. Не трогаем.
- **QR-скан**: `qrRoute` (`qr_camera.go:178`) — единственный, кто меняется.

Поэтому различие структурное: достаточно **перестать** пускать non-IO результат QR в
`textFromIntent`. Share-путь (`for_android.go`) и потребитель (`send.go:1643`) не меняются —
корзина продолжает получать Share-текст. `uriFromIntent` смешивает IO/URI от обоих источников, но
их обработка (`fromURI`/relay) одинакова → безразлично.

## Решение (зафиксировано: прямой путь — Вариант A)
В `qrRoute` ветку non-IO заменить на прямую отправку в UI через `fyne.Do`: `qr.SetClipboard(text)`
(буфер + `UpdateFromClipboard` → QR + link + видимость) + `qr.Show()` (вкладка QR в настройках,
`applinks.go:277`: `at.SelectIndex(3)` + открыть аккордеон «QR»). Новый канал не нужен.

Отображение уже обеспечивает секция QR (`updateLink`, `applinks.go:311`):
- `IO` → `handleIOTapped` (резолв диплинка);
- парсится как URL → кликабельно, `OpenDAV` → `OpenURL`/браузер (`for_android.go:682`);
- иначе → `showTextDialog` (показать текст).

## Реализация — `qr_camera.go`

### `qrRoute` (≈166–187)
Ветку `else` (non-IO):
```go
} else {
    select {
    case textFromIntent <- text:
    default:
        log.Debug("textFromIntent full")
    }
}
```
заменить на:
```go
} else {
    fyne.Do(func() {
        if qr != nil {
            qr.SetClipboard(text) // буфер обмена + UpdateFromClipboard (QR/link/видимость)
            qr.Show()             // открыть вкладку QR в настройках
        } else {
            log.Debug("qrRoute: qr not built, skipping clipboard/Show")
        }
    })
}
```
Ветка `IO` (`uriFromIntent`) и колбэк `cb` — без изменений.

### Без изменений
- `for_android.go` `intentTextNotify`/`intentURINotify` (Share → `textFromIntent`/`uriFromIntent`).
- `send.go:1643` потребитель `textFromIntent` (корзина передачи) — по-прежнему для Share.
- `applinks.go` `QR` (`SetClipboard`, `UpdateFromClipboard`, `updateLink`, `Show`), колбэк сканера
  настроек (`applinks.go:442` — для non-IO избыточен, но нужен для отображения IO-ссылок; оставляем).
- `recv.go:118` (`startQRScan(a,w,nil)` — колбэк nil).

## Поведение
- **QR-скан non-IO** (с recv и с настроек): буфер обмена + переход на вкладку QR; ссылка кликабельна,
  иначе текст. В корзину передачи **не** попадает.
- **QR-скан IO**: как раньше — `uriFromIntent` → секрет/relay.
- **Share-текст/URI из другого приложения**: как раньше — корзина/`fromURI`.
- **Браузерный/интент-сканер (non-builtin)**: не затрагивается (результат через буфер на resume
  шлёт в `uriFromIntent` только IO-префикс, `send.go:1620-1625`).

## Риски
- `qrRoute` вызывается из decode-горутины; UI-операции — через `fyne.Do`; `qr` — package-global
  (`applinks.go:968`), создаётся на старте (`settings.go:499`), nil-защита добавлена.
- Колбэк сканера настроек для non-IO дублирует `updateLink`/`Show` — безвредно; оставляем (нужен для IO).
- `textFromIntent` остаётся живым (питается Share) — мёртвым кодом не становится.

## Валидация
1. **Компиляция:** `make arm64 wsl`.
2. **Рантайм (Android, builtin-сканер):**
   - Открыть QR-скан с **recv** и с **настроек** → навести на non-IO QR (текст и ссылку): текст в
     буфере, открыта вкладка QR; под QR ссылка кликабельна (браузер), не-ссылка → текст-диалог; в
     корзину передачи ничего не падает.
   - Навести на **IO**-QR → секрет/relay заполнились (как раньше), в корзину не падает.
   - Поделиться текстом из другого приложения (Share) → по-прежнему попадает в корзину передачи.
   - `make atags`/gopls для Android-тега при необходимости.
