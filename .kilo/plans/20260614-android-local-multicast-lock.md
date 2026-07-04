# Почему `--local` не находит пиров на Android (диагноз + план фикса)

## Диагноз (главное)

В логе Android:
```
croc.go:1075: discoveries: []            <- пусто
croc.go:1104: could not connect to : found no addresses to connect
```
`discoveries` пустой, потому что на Android приём **multicast** UDP-пакетов по
умолчанию **блокируется** wifi-драйвером.

### Как устроен discovery в croc
`croc --local` использует библиотеку `github.com/schollz/peerdiscovery`:
- отправитель широковещательно шлёт свой payload (`croc<port>`) на
  multicast-группу **`239.255.255.250:9999`** (IPv4) / `ff02::c` (IPv6);
- получатель через `ipv4.JoinGroup` вступает в группу и слушает эти пакеты
  (`src/abakCroc/croc/...` → `src/croc/croc.go:1014` `peerdiscovery.Discover`,
  в библиотеке — `../peerdiscovery/listener.go:114` `p2.JoinGroup`).

На WSL/десктопе ОС доставляет multicast в сокет — discovery проходит (видно в
вашем логе `discoveries: [address: 172.22.208.1 ...]`).
На Android же, чтобы ядро передавало multicast-датаграммы приложению, нужно:

1. разрешение **`android.permission.CHANGE_WIFI_MULTICAST_STATE`** в манифесте;
2. активный **`WifiManager.MulticastLock`** (acquired) на всё время discovery.

Ни того, ни другого в проекте **нет**:
- `AndroidManifest.xml` (`grep CHANGE_WIFI` → ничего);
- в коде нет `MulticastLock`/`createMulticastLock` ни в Java, ни в Go.

Без MulticastLock `peerdiscovery.Discover` вступает в группу, но ни один пакет
до сокета не доходит → `discoveries: []` → ошибка `found no addresses to connect`.

Это объясняет ровно наблюдаемую картину: WSL работает, Android — нет.

## План фикса

### 1. Манифест
`AndroidManifest.xml` (после строки 110 `<uses-permission ...FOREGROUND_SERVICE_DATA_SYNC/>`):
```xml
<uses-permission android:name="android.permission.CHANGE_WIFI_MULTICAST_STATE" />
```
(обычного, не runtime-разрешения достаточно — оно normal-protection.)

### 2. Java: acquire/release MulticastLock
В `GoNativeActivity.java` добавить два static-метода (рядом с другими
`static void ...`, напр. после `startCrocsonService`/`stopCrocsonService`,
строки ~170-194). Импортировать `android.net.wifi.WifiManager` и
`android.content.Context`.

```java
private static WifiManager.MulticastLock multicastLock;

static void acquireMulticastLock() {
    try {
        if (multicastLock != null && multicastLock.isHeld()) return;
        Context ctx = goNativeActivity != null ? goNativeActivity : null;
        if (ctx == null) return;
        WifiManager wm = (WifiManager) ctx.getSystemService(Context.WIFI_SERVICE);
        if (wm == null) return;
        multicastLock = wm.createMulticastLock("croc");
        multicastLock.setReferenceCounted(false);
        multicastLock.acquire();
    } catch (Throwable t) {
        Log.e(TAG, "acquireMulticastLock", t);
    }
}

static void releaseMulticastLock() {
    try {
        if (multicastLock != null && multicastLock.isHeld()) {
            multicastLock.release();
        }
    } catch (Throwable t) {
        Log.e(TAG, "releaseMulticastLock", t);
    }
}
```
Убедиться, что поле `goNativeActivity` инициализируется (оно уже есть как
`private static GoNativeActivity goNativeActivity;`, проверим в `onCreate`).

### 3. Go (android): вызывать через существующий JNI-мост
В `for_android.go` **уже есть** `callVoid(method string)` (`for_android.go:37`),
который через C вызывает static-метод на классе активности. Добавить обёртки:

```go
func acquireMulticastLock()  { _ = callVoid("acquireMulticastLock") }
func releaseMulticastLock()  { _ = callVoid("releaseMulticastLock") }
```

### 4. Точки вызова и УСЛОВИЕ запуска

**Условие:** multicast-lock приобретается только когда croc реально запускает
discovery. Discovery в croc запускается по условию
`if !c.Options.DisableLocal && !isIPset` (`src/croc/croc.go:1006`). У получателя
`isIPset` практически = false (IP не задаётся вручную), поэтому рабочий гейт —
**`!opt.DisableLocal`** (т.е. «local не отключён»). Это строго шире, чем `OnlyLocal`,
и совпадает с тем, когда discovery действительно бежит. Узко вешать на `OnlyLocal`
нельзя — пропустит обычный режим, где discovery тоже работает.

Различать две роли:
- **Получатель** (`recv.go`, блок `opt := croc.Options{...}` ~`recv.go:808-838`,
  далее `crocNew`/`client.Receive` ~`:1011`): здесь критично получить multicast-анонс
  отправителя → `if !opt.DisableLocal { acquireMulticastLock(); defer releaseMulticastLock() }`.
  acquire ставить **сразу после сборки `opt`** и **до** `crocNew`/`Receive`
  (чтобы лок держался во время всего discovery + установки соединения);
  release — по завершении/отмене receive.
- **Отправитель** — MulticastLock **НЕ ставим** (по решению): рассылка multicast
  идёт и без лока, а для `--local` отправителя это не требуется.

### 4a. WebDAV / туннели (wormhole, webwormhole) — блок НЕ трогаем

Multicast discovery запускается **только** в croc-ветви получателя. Структура
кнопки Download в `recv.go`:
- `if davServer.IsLocal()` → возврат (`recv.go:602`);
- `if isWormholeRelay(relayAddr)` → путь wormhole, `return` (`recv.go:631`);
- `if isWebWormholeRelay(relayAddr)` → путь webwormhole, `return` (`recv.go:716`);
- иначе → croc-путь: `opt := croc.Options{}` (`recv.go:808`) + `client.Receive()`
  (`recv.go:1011`) — **единственная** ветвь с multicast.

WebDAV/wormhole/webwormhole используют собственный транспорт
(mailbox/transit/websocket-relay) и **не** используют `peerdiscovery`/multicast,
поэтому `CHANGE_WIFI_MULTICAST_STATE`/`MulticastLock` для них не нужен и acquire
туда не добавляется. Достаточно, что acquire стоит только в croc-ветви.

Реализация на Go-стороне crocson (build-tag `android`): в `for_android.go`
определить `acquireMulticastLock()`/`releaseMulticastLock()` (см. п.3) и
no-op-заглушки в `for_mobile.go`/`for_unix.go`/`for_windows.go` (чтобы вызовы из
общего кода компилировались на всех платформах). В `recv.go`/`send.go` вызывать
без build-tagов — выбирается нужная реализация через файлы `for_*.go`.

Точные места вставки в `recv.go`/`send.go` уточнить при имплементации по контексту
вокруг `crocNew`/`client.Receive`/`client.Send`.

## Результат тестирования

### Эмулятор (android/amd64) — фикc работает, но discovery пустой (ожидаемо)
Лог подтверждает корректное поведение фикса:
```
09:10:12 recv.go:850: croc client created
09:10:12 Java: MulticastLock acquired        <- лок взят
09:10:13 croc.go:1007: attempt to discover peers
09:10:13 croc.go:1075: discoveries: []        <- пусто
09:10:13 Java: MulticastLock released         <- лок отдан
```
Лок удерживается ровно во время discovery и освобождается в cleanup-`defer`.
Однако `discoveries: []` остаётся пустым, потому что **эмулятор** работает в
виртуальной NAT-сети (router между эмулятором и хостом WSL), через которую
multicast `239.255.255.250:9999` между хостом и эмулятором **не пробрасывается**
в принципе. MulticastLock тут бессилен — пакеты не доходят до wifi-стека эмулятора.
Реальная проверка — только на физическом телефоне в одной Wi-Fi-сети с отправителем.

### Физический телефон — TODO
Ожидаемый успех: `MulticastLock acquired` → `discoveries: [...]` (непусто) →
`switching to local` → соединение устанавливается. Если и тут пусто — причины:
AP isolation на роутере, разные подсети Wi-Fi (2.4/5GHz isolation), либо
firewall блочит UDP 9999/multicast. Тогда discovery не сработает, но croc
использует фолбэк через relay/ipRequest (`transferOverLocalRelay`, croc.go:631).

## Кто реально нуждается в MulticastLock

`peerdiscovery.Discover` симметричен: **обе** стороны делают `net.ListenPacket`
на multicast-адресе, `JoinGroup` по всем интерфейсам и `go p.listen(c)`
(`../peerdiscovery/listener.go:113-115`, `peerdiscovery.go:150-170`). Различаются
только `Limit`/payload:
- отправитель: `Limit: -1`, шлёт `croc<port>`, тоже слушает (видно в логе
  `discoveries: [... payload: ok]`);
- получатель: `Limit: 1`, шлёт `ok`, слушает и **ему критично** получить
  `croc<port>` от отправителя.

Но `MulticastLock` нужен **только для приёма** multicast-датаграмм; **отправлять**
multicast лок держать не требуется. Поэтому:
- **Android = получатель** → acquire/release обязателен (иначе `discoveries: []`);
- Android = отправитель → **не делаем** (по решению): multicast-рассылка работает без лока.

Практически: acquire/release ставится **только** на croc-ветви получателя в `recv.go`
(под условием `!opt.DisableLocal`). Ветви WebDAV / wormhole / webwormhole не трогаются
(см. 4a), на `send.go` — ничего не ставим.

## Альтернативы / заметки
- Если MulticastLock не поможет (некоторые роутеры/AP изоляция), discovery
  всё равно не сработает, но тогда croc использует фолбэк через relay/ipRequest
  (см. комментарий `transferOverLocalRelay`, `croc.go:631`), т.е. это
  самостоятельный отказ только discovery, а не всей передачи.
- IPv6 discovery на Android часто дополнительно проблематичен; фикс в первую
  очередь для IPv4 (`239.255.255.250`).

## Файлы для правки
- `AndroidManifest.xml` (+1 permission — декларируется всегда, безвредно вне local)
- `GoNativeActivity.java` (+2 static-метода, +импорт WifiManager/Context)
- `for_android.go` (+2 обёртки `acquireMulticastLock`/`releaseMulticastLock`)
- `for_mobile.go` / `for_unix.go` / `for_windows.go` (no-op-заглушки тех же функций)
- `recv.go` (acquire/release под условием `!opt.DisableLocal`, **только** в croc-ветви получателя)
- WebDAV/wormhole/webwormhole ветви в `recv.go` — **не трогать** (multicast не используют)
- `send.go` — **не трогать** (по решению multicast-lock на отправителе не нужен)
