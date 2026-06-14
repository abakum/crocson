# Fix: ForegroundServiceDidNotStartInTimeException — проверка условия + страховка-дебаунс

## Root Cause

`startForegroundService()` и `stopService()` вызываются с интервалом ~11мс, **до** того как
сервис успевает создаться:

```
07:11:14.739  Foreground service started        <- startForegroundService()
07:11:14.750  Foreground service stopped        <- stopService()  (до onCreate!)
07:11:14.751  CrocsonService onCreate
07:11:14.757  startForeground called
07:11:14.758  onDestroy
07:11:14.759  FATAL: ForegroundServiceDidNotStartInTimeException
```

`caffeinate(1)` зовётся **оптимистично** в самом начале горутины прогресса (`recv.go:876`),
до того как известно, что приём состоится. Приём тут же падает ("found no addresses to
connect"), defer зовёт `caffeinate(-1)` — старт→стоп за 11мс.

## Решение (две части)

### Часть A — условие: стартовать сервис только при реальном переносе

Перенести `caffeinate(1)` в точку подтверждения переноса; спарить с `caffeinate(-1)` через
локальный флаг `caffeinated`.

#### A1. recv.go — горутина прогресса (место краша, стр. 874-901)

- Убрать `caffeinate(1)` со стр. 876.
- Добавить `caffeinated := false` рядом с `once := true` (стр. 909).
- В блоке `once` (стр. 942-943, условие `client.Step2FileInfoTransferred`):
  ```go
  once = false
  caffeinated = true
  caffeinate(1)
  ```
- В defer (стр. 877-900) защитить:
  ```go
  if caffeinated {
      caffeinate(-1)
  }
  ```

#### A2. send.go — горутина прогресса croc-отправки (стр. 1408-1434)

- Убрать `caffeinate(1)` со стр. 1410.
- Добавить `caffeinated := false` рядом с `once := true` (стр. 1443).
- В блоке `once` (стр. 1467-1469, условие `hashed(client)`):
  ```go
  once = false
  caffeinated = true
  caffeinate(1)
  ```
- Защитить `caffeinate(-1)` в defer (стр. 1416).

#### A3. send.go — wormhole-горутина webwormhole (стр. ~1005-1078)

- Убрать `caffeinate(1)` со стр. 1041.
- Добавить `caffeinated := false` в начале горутины.
- Перенести старт **после** успешного соединения, после `wt.tunnel = t` (стр. 1073):
  ```go
  wt.tunnel = t
  caffeinated = true
  caffeinate(1)
  ```
- Защитить `caffeinate(-1)` в defer (стр. 1022): `if caffeinated { caffeinate(-1) }`.

#### A4. send.go — wormhole-горутина (стр. ~1135-1206)

Аналогично A3:
- Убрать `caffeinate(1)` со стр. 1167.
- `caffeinated := false` в начале.
- Перенести старт после `wt.tunnel = t` (стр. 1201).
- Защитить `caffeinate(-1)` в defer (стр. 1148).

#### A5. webdav.go — без изменений

WebDAV-сервер длительно работающий, гонка практически исключена.

### Часть B — страховка: мини-дебаунс остановки в for_android.go

Перехватывает любой остаточный случай (перенос начался и оборвался за <100мс), независимо от
сайта вызова.

В `for_android.go`, рядом с `sleepCounter`:
```go
var (
	sleepCounter int32
	fgStopMu     sync.Mutex
	fgStopTimer  *time.Timer
)
```

Переписать `caffeinate` (стр. 537-555):
```go
func caffeinate(i int32) int32 {
	old := atomic.LoadInt32(&sleepCounter)
	var newVal int32

	if i == 0 {
		atomic.StoreInt32(&sleepCounter, 0)
		newVal = 0
	} else {
		newVal = atomic.AddInt32(&sleepCounter, i)
	}

	if old <= 0 && newVal > 0 {
		fgStopMu.Lock()
		if fgStopTimer != nil {
			fgStopTimer.Stop()
			fgStopTimer = nil
		}
		fgStopMu.Unlock()
		startForegroundService()
	} else if old > 0 && newVal <= 0 {
		fgStopMu.Lock()
		if fgStopTimer != nil {
			fgStopTimer.Stop()
		}
		if i == 0 {
			fgStopTimer = nil
			fgStopMu.Unlock()
			stopForegroundService()
		} else {
			fgStopTimer = time.AfterFunc(3*time.Second, func() {
				stopForegroundService()
			})
			fgStopMu.Unlock()
		}
	}

	return newVal
}
```

Добавить `"sync"` в блок импорта (сейчас есть только `sync/atomic`).

## Файлы

1. `recv.go` — A1 (краш).
2. `send.go` — A2, A3, A4.
3. `for_android.go` — часть B (импорт `sync`, vars, переписать `caffeinate`).

## Проверка

- Воспроизвести краш: приём без пира → сервис **вообще не стартует**, краша нет.
- Нормальный перенос: сервис стартует при начале передачи, останавливается ~3с после.
- `go vet -tags android ./...`
