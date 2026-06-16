# План: починить локальную доставку (local discovery) Windows → Android

## Решение
**Вариант A — правка в форке `abakum/peerdiscovery`** (root-cause). Правка и публикация форка выполняются пользователем самостоятельно.

## Корневая причина (кратко)
Android получает собственное multicast-эхо (`payload: ok`) с числовым zone `fe80::...%24`. Self-фильтр peerdiscovery (`listener.go:135`) его не отбрасывает, т.к. `getLocalIPs()` (`internal.go`) формирует ключи только `ip.String()` и `ip.String()+"%"+iface.Name`, а ядро отдаёт числовой `%<index>`. Это эхо закрывает квоту `Limit:1`, listener останавливается и не успевает считать `croc9009` от отправителя.

Причастность `abakum/anet`: `interface_android.go:29-33` сознательно удалил `zoneCache` (`go:linkname`) для Go 1.25+, и `newAddr` возвращает IPv6 link-local без zone → peerdiscovery физически не может породить числовой zone.

## Правка (на стороне пользователя, форк peerdiscovery)
Файл `internal.go`, функция `getLocalIPs()`, цикл по адресам:
```go
ips[ip.String()+"%"+iface.Name] = struct{}{}
ips[ip.String()+"%"+strconv.Itoa(iface.Index)] = struct{}{} // ← ДОБАВИТЬ
ips[ip.String()] = struct{}{}
```
- `iface.Index` доступен (anet `newLink` выставляет `Index: int(ifam.Index)`).
- Импорт `strconv` уже есть в `internal.go` (используется в `initialize`).
- Базовый коммит для правки: fast-forward локального `../peerdiscovery` на pinned `7a998a1dc036` (локальный HEAD сейчас отстаёт — `bda3939`).

## Интеграция в crocson (выполняет пользователь)
После пуша форка — обновить `replace` в `go.mod` crocson:
```
replace github.com/schollz/peerdiscovery => github.com/abakum/peerdiscovery <новая-pseudo-version>
```
Затем `go mod tidy` и `make wsl`.

## Можно ли поправить anet, не теряя совместимости с Go 1.25? (анализ — НЕТ смысла)

Два разных вопроса смешивать нельзя:

### 1) Вернуть linkname→stdlib zoneCache — нельзя.
anet (коммит `26109fc`) удалил `//go:linkname zoneCache net.zoneCache` и `zoneCacheX ...socket.zoneCache`, потому что Go 1.25 по умолчанию собирается с `-checklinkname=0`, и такие `linkname` блокируют сборку. Публичного API, чтобы «скормить» свой интерфейсный кэш во внутренний `net.zoneCache`/`socket.zoneCache`, нет — только `linkname`. Т.о. синхронизацию anet→stdlib zone-cache восстановить на 1.25 без потери совместимости **нельзя**.

### 2) Заставить anet возвращать link-local с zone через публичный API — можно, но это всё сломает.
В `newAddr` (interface_android.go:331) доступен `ifam.Index`, поэтому zone можно прицепить руками (напр. через `net.IPAddr{Zone: strconv.Itoa(int(ifam.Index))}`). **НО** Go 1.25 `net.ParseCIDR` явно отвергает адреса с zone (GOROOT `src/net/ip.go:557`):
```go
ipAddr, err := netip.ParseAddr(addr)
if err != nil || ipAddr.Zone() != "" {
    return nil, nil, &ParseError{Type: "CIDR address", Text: s}
}
```
А peerdiscovery `getLocalIPs` парсит адреса именно через `net.ParseCIDR(address.String())`. Значит link-local с zone вообще не попадут в `localIPs` → self-фильтр станет ещё хуже, и обнаружение сломается окончательно. Так делать **нельзя**.

### Вывод
anet правка для нашего бага **не нужна и вредна**. anet уже отдаёт `iface.Index` (через `Interfaces()`→`newLink`), и этого достаточно, чтобы peerdiscovery сам построил числовой zone-ключ в `getLocalIPs`. Фикс — исключительно Вариант A в peerdiscovery, без изменений anet и без `linkname`.

## Что НЕ меняется в коде crocson
Никаких правок исходников crocson (var-правка recv.go про `OnlyLocal` уже применена ранее и остаётся).

## Риск / проверка
- Кросс-устройственная доставка Windows→Android IPv6 multicast не гарантирована self-эхом (последнее доказывает лишь RX на Android). После правки повторить тест; ожидаемое в логе получателя:
  `discovery ... payload: croc9009` → `switching to local` → `successfully pinged '<ip>:9009'` → `receiver connection established`.
- Если `croc9009` так и не придёт — проблема сетевая (файрвол/подсеть/IPv4-multicast, который в логе пуст), кодом не лечится; рассматривать fallback на relay.
