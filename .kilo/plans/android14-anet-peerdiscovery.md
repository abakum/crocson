# Android 14 local discovery: починить через anet в форке peerdiscovery

## Первопричина (найдена точно)
`peerdiscovery` строит список интерфейсов через **stdlib `net.Interfaces()`** и
`iface.Addrs()` (`internal.go:66,80,120,126`, `listener.go:99`). На **Android 11+
(API ≥ 30)** stdlib `net.Interfaces()` ломается: Go читает `/proc/net/if_inet6` и
использует `getifaddrs`, которые в network-namespace/ограничениях SELinux
non-system приложения возвращают **пустой/битый список** на новых Android.

⇒ `filterInterfaces` возвращает пусто ⇒ `newPeerDiscovery` (`peerdiscovery.go:144`)
возвращает ошибку `no multicast interface found` ⇒ croc-горутина глушит её
(`croc.go:1022`) ⇒ `discoveries: []` и **мгновенный возврат (~2мс)** — ровно то,
что в логах. На Android 9 (API 28 < 30) stdlib ещё работает → discovery проходит.

В crocson уже подключён форк **`abakum/anet`** (`interface_android.go:36`):
```go
func Interfaces() ([]net.Interface, error) {
    if androidApiLevel() < 30 { return net.Interfaces() }  // Android <11
    return interfaceTable(0)  // Android ≥11: netlink RTM_GETADDR
}
```
`anet` чинит ровно эту проблему, **но только там, где его вызывают явно** — а
`peerdiscovery` вызывает stdlib `net`. Поэтому `anet` (доступный через pion/
webwormhole) discovery не помогал.

Это объясняет ВСЕ наблюдения:
- физический Android 9 — работает (stdlib ок + MulticastLock);
- физический Android 14 — пусто (stdlib сломан; нужен anet);
- эмуляторы — пусто (NAT, отдельная причина).

MulticastLock (сделанный ранее) остаётся нужным (чтобы ядро доставляло multicast в
сокет), но был necessary-not-sufficient: на Android 14 ещё и интерфейсы не
перечислялись.

## План фикса

### 1. Форк `abakum/peerdiscovery` (`/home/koka/src/peerdiscovery`)
Заменить stdlib-перечисление интерфейсов на `anet` (drop-in: `anet.*` возвращает те
же типы `[]net.Interface` / `[]net.Addr`).

1.1. `go.mod`: добавить зависимость
```go
require github.com/wlynxg/anet v0.0.5
```
(версия wlynxg/anet — в crocson replaceredirectит на abakum/anet).

**Почему без `replace` в `peerdiscovery/go.mod`:**
- `replace`-директивы применяет **только главный модуль** (crocson); в go.mod
  зависимости (peerdiscovery) они игнорируются при сборке потребителя.
- Форк `abakum/anet` держит канонический module path `module github.com/wlynxg/anet`
  (не `github.com/abakum/anet`) ⇒ require/импорты — всегда `github.com/wlynxg/anet`,
  редирект на форк — на уровне crocson (`replace github.com/wlynxg/anet => ../anet`).
- Опционально: если хочется, чтобы и самостоятельные `go test`/`go build` внутри
  репо форка peerdiscovery тоже резолвили anet-форк, можно добавить локальный
  `replace github.com/wlynxg/anet => ../anet` в `peerdiscovery/go.mod` — для сборки
  crocson это нейтрально.

1.2. `internal.go`:
- импорт: добавить `"github.com/wlynxg/anet"` (оставить `"net"` для типов
  `net.Interface`/`net.Addr`/`net.Flags`/`net.IPNet`).
- `filterInterfaces` (`:66`): `allIfaces, err := net.Interfaces()` →
  `anet.Interfaces()`.
- `filterInterfaces` (`:80`): `addrs, addrsErr := iface.Addrs()` →
  `addrs, addrsErr := anet.InterfaceAddrsByInterface(&iface)`.
- `getLocalIPs` (`:120`): `ifaces, err := net.Interfaces()` → `anet.Interfaces()`.
- `getLocalIPs` (`:126`): `addrs, err := iface.Addrs()` →
  `anet.InterfaceAddrsByInterface(&iface)`.

1.3. `listener.go` (`:99`): `ifaces, err := net.Interfaces()` → `anet.Interfaces()`.

`broadcast()`/`peerdiscovery.go` (используют `net.Interface` как тип) — БЕЗ изменений.

### 2. crocson — переключить на локальные форки
Выполнить `make local` (Makefile:61-66), что выставит replaces на локальные пути:
- `github.com/schollz/peerdiscovery => ../peerdiscovery`
- `github.com/wlynxg/anet => ../anet`

(Либо вручную `go mod edit -replace=...`.) Это позволит собрать с изменённым
форком без публикации. После проверки — опубликовать форк peerdiscovery и вернуть
`make`-replace на опубликованную версию (как для anet).

### 3. Сборка и проверка
- `make amd64` → APK.
- `make adb` → установка.
- **Тест на физическом Android 14** (sender = WSL/Windows):
  - ожидаемо: `attempt to discover peers` → через ~десятки/до 200мс
    `discoveries: [address: <sender>, payload: croc9009]` → `switching to local`.
  - эмуляторы по-прежнему пусто (NAT) — это нормально.

### 4. Диагностика (если на физическом Android 14 всё ещё пусто)
Тайминг `attempt → discoveries`:
- стало ~до 200мс (раньше ~2мс) ⇒ anet сработал, интерфейсы нашлись; дальше если
  пусто — уже сценарий B (дроп multicast драйвером/AP) → WifiLock high-perf +
  проверка AP isolation.
- осталось ~2мс ⇒ anet не подхватился/возвращает пусто — проверить, что в бинарник
  попала Android-сборка anet (cgo API-level detection) и что replace применился
  (`go list -m github.com/schollz/peerdiscovery`).

## Файлы для правки
- `/home/koka/src/peerdiscovery/go.mod` (+require wlynxg/anet)
- `/home/koka/src/peerdiscovery/internal.go` (4 замены net→anet)
- `/home/koka/src/peerdiscovery/listener.go` (1 замена net→anet)
- `/home/koka/src/crocson/go.mod` (через `make local` — replaces на локальные пути)

## Риск / заметки
- `anet` cross-platform: на не-Android делегирует в stdlib `net`, так что WSL/
  Windows/десктоп поведение не меняется.
- `&iface` в range-цикле безопасно (локальная копия; работает на всех версиях Go).
- MulticastLock (уже реализован) + этот фикс = полный набор для local discovery на
  Android 11+.
