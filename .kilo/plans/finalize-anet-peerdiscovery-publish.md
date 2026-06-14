# Финализация: публикация форка peerdiscovery + возврат replaces на опубликованные версии

## Статус (что сделано и подтверждено)
Локальная сборка с локальными форками работает: на физическом Android 14 local
discovery находит пира, передача проходит (`hashes are equal`), MulticastLock
корректно acquired/released. Комбинация правок:
- **crocson**: AndroidManifest (+`CHANGE_WIFI_MULTICAST_STATE`), GoNativeActivity
  (+`acquireMulticastLock`/`releaseMulticastLock` `boolean`), for_android.{go,c,h}
  (+`callBoolean`), multicast_lock_other.go (no-op), recv.go (acquire/release под
  `!DisableLocal`).
- **peerdiscovery форк**: `net.Interfaces()`/`iface.Addrs()` → `anet.*` (4 точки
  internal.go + 1 listener.go) + anet в go.mod.

## Что осталось (релизные хвосты)

### 1. Опубликовать форк peerdiscovery (`/home/koka/src/peerdiscovery`)
Сейчас изменения только локально (`git status`: `M go.mod internal.go listener.go`,
последний коммит `bda3939 gc return` — БЕЗ anet).

1.1. В репо форка прогнать `go mod tidy` (добавит go.sum-запись для wlynxg/anet).
1.2. Закоммитить: `go.mod`, `go.sum`, `internal.go`, `listener.go`
     (сообщение вида: `feat: use anet for interface enumeration (fix Android 11+)`).
1.3. `git push origin main` → `github.com/abakum/peerdiscovery`.

> Примечание: `replace` в `peerdiscovery/go.md` НЕ нужен (replace работает только
> из главного модуля; редирект wlynxg/anet→abakum/anet живёт в crocson).

### 2. crocson: вернуть replaces на опубликованные форки
Сейчас все replaces — локальные (`make local` отработал): `../peerdiscovery`,
`../anet`, `../abakCroc/croc`, и т.д. Для релиза — `make repo` (Makefile:40-57),
он переключит ВСЕ replaces на опубликованные версии:
- `peerdiscovery => github.com/abakum/peerdiscovery@<main>` (новый, с anet —
  поэтому пушить п.1 ОБЯЗАТЕЛЬНО до этого шага);
- `anet => github.com/abakum/anet@<main>` (уже опубликован);
- croc/wormhole-william/webwormhole — на их опубликованные форки.

Затем `make repo` сам выполнит `go mod tidy`.

### 3. Финальная сборка/проверка
- `make amd64` → APK.
- Проверить `go list -m github.com/schollz/peerdiscovery` — должен показать
  опубликованную версию (не `=> ../peerdiscovery`).
- `make adb` → установка.
- (опц.) повторный тест на физическом Android 14 — должен работать идентично
  локальной сборке.

## Проверить (не my-changes, но в `git status`)
В crocson `git status` показывает также `M FyneApp.toml`, `M send.go` — это не
мои правки (сессия их не трогала). Перед коммитом crocson убедиться, что эти
изменения ожидаемы/корректны (или вынести отдельно).

## Файлы (crocson), подлежащие коммиту после финализации
- AndroidManifest.xml
- GoNativeActivity.java
- for_android.c / for_android.go / for_android.h
- multicast_lock_other.go (новый)
- recv.go
- go.mod / go.sum (после `make repo`)
- (отдельно, по решению) FyneApp.toml, send.go
