# Выделенный WS для автооткрытия браузера по чату (локально + через туннель)

## Постановка

`wsRefreshRemote` — **только для оповещения об обновлении дерева файлов**. Его **не трогаем**.
Автооткрытие браузера по чату сейчас висит на `wsRefreshRemote` (`send.go:729-735`, параметр
`onChatMessage`), но этот WS подключается к `proxyURL` и **убивается внутренним guard'ом**
`wsRefreshRemote` (`send.go:77`: `!IsRemote() && !IsTCPForwardingActive()`) на отправителе
(без туннеля) ещё до `Dial`. → сценарий «получатель шлёт первым → автооткрытие у отправителя»
не работает. Это регрессия коммита `c136659`, который объединил чат с refresh, но не учёл
внутренний guard (прототип на long-poll был отдельной горутиной без такого guard'а и работал).

Нужно: **отдельный WS для чата**, который подключается к `proxyURL` (как работавший прототип
long-poll) и не подчиняется guard'у `wsRefreshRemote`. `proxyURL` достижим на обеих сторонах:
- **отправитель:** `proxyURL` (ccn) — локальный WebDAV-сервер;
- **получатель:** `proxyURL` = `http://127.0.0.1:8080` — TCP-forwarder → туннель → сервер
  отправителя (см. лог: `ccn="http://127.0.0.1:8080"`).

---

## Изменения

### 1. `send.go` — откатить вмешательство в `wsRefreshRemote`

- **`send.go:77`** — вернуть guard в исходный вид (убрать добавленное `!davServer.IsActive()`):
  ```go
  if !davServer.IsRemote() && !davServer.IsTCPForwardingActive() {
      log.Debugf("[ws-refresh] tunnel down, stopping")
      return
  }
  ```
- **`send.go:728-735`** — убрать автооткрытие из вызова `wsRefreshRemote`: 4-й аргумент
  (`onChatMessage`) передать `nil` (тело `wsRefreshRemote` и его параметр не меняем —
  «не трогать»; `if onChatMessage != nil` остаётся no-op):
  ```go
  wsRefreshRemote(wsCtx, proxyURL.String(), onRefresh, nil)
  ```

### 2. `send.go` — новая функция `wsChat` (рядом с `wsRefreshRemote`, ~`send.go:63`)

WS-клиент только для чата: подключается к `<httpURL>/api/chat/ws`, переподключается,
игнорирует историю (`[`) и команды `refresh`/`close`, на чат-сообщение зовёт `onMessage`.
Скелет — как у `wsRefreshRemote`, но с guard'ом, пропускающим отправителя:

```go
func wsChat(ctx context.Context, httpURL string, onMessage func()) {
	wsScheme := "ws"
	if strings.HasPrefix(httpURL, "https") {
		wsScheme = "wss"
	}
	wsURL := strings.Replace(httpURL, "http", wsScheme, 1) + "/api/chat/ws"
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// proxyURL достижим, пока активен локальный сервер (отправитель)
		// или TCP-форвардер (получатель)
		if !davServer.IsActive() && !davServer.IsTCPForwardingActive() {
			log.Debugf("[ws-chat] server down, stopping")
			return
		}
		dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		conn, _, err := dialer.Dial(wsURL, nil)
		if err != nil {
			log.Debugf("[ws-chat] dial error: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
				continue
			}
		}
		log.Debugf("[ws-chat] connected to %s", wsURL)
		connDone := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				conn.Close()
			case <-connDone:
			}
		}()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				close(connDone)
				conn.Close()
				log.Debugf("[ws-chat] read error: %v", err)
				break
			}
			if len(data) > 0 && data[0] == '[' {
				continue // история
			}
			var cmd struct{ Cmd string `json:"cmd"` }
			if json.Unmarshal(data, &cmd) == nil && (cmd.Cmd == "refresh" || cmd.Cmd == "close") {
				continue
			}
			if onMessage != nil {
				onMessage()
			}
		}
	}
}
```
`strings`, `time`, `tls`, `websocket`, `json`, `context` — всё уже импортировано в `send.go`.

> Guard `!IsActive() && !IsTCPForwardingActive()` — ключевое отличие от `wsRefreshRemote`:
> проходит и для отправителя (`IsActive()==true`), и для получателя (`IsTCPForwardingActive()==true`).
> Это и восстанавливает свойство прототипа «подключается всегда на обеих сторонах».

### 3. `send.go` — общий гейт «браузер уже открывали» (scope `sendTabItem`)

В блоке переменных `sendTabItem` (после `cancelWS`, `send.go:147`) добавить:

```go
var (
	cancelChatWS context.CancelFunc

	browserOpenedMu sync.Mutex
	browserOpened   bool
)

// openBrowser открывает URL один раз за сессию чата; true, если открыл.
openBrowser := func(url string) bool {
	browserOpenedMu.Lock()
	if browserOpened {
		browserOpenedMu.Unlock()
		return false
	}
	browserOpened = true
	browserOpenedMu.Unlock()
	OpenURL(url)
	return true
}
```

### 4. `send.go` — стартер `startChatWS`

В scope `sendTabItem` (до `switchToWebDAVTree`). `proxyURL`/`chatURL` уже выставлены в
`switchToWebDAVTree` (`send.go:707-709`), поэтому URL не вычисляем — передаём `proxyURL`,
а браузер открываем на `chatURL` (= ccn):

```go
startChatWS := func(httpURL string) {
	if httpURL == "" {
		return
	}
	if !davServer.IsActive() && !davServer.IsTCPForwardingActive() {
		return
	}
	if cancelChatWS != nil {
		cancelChatWS()
	}
	browserOpenedMu.Lock()
	browserOpened = false // новая сессия → допускаем одно открытие
	browserOpenedMu.Unlock()

	var ctx context.Context
	ctx, cancelChatWS = context.WithCancel(appCtx)

	go wsChat(ctx, httpURL, func() {
		if chatURL != "" && openBrowser(chatURL) {
			log.Debugf("[ws-chat] auto-opening browser: %s", chatURL)
		}
	})
}
```

### 5. `send.go` — запуск `startChatWS` для обеих сторон

В горутине `switchToWebDAVTree` (`send.go:710`, после создания `wsCtx`) добавить запуск
выделенного чат-WS (он независим от `wsRefreshRemote`/дерева):

```go
go func() {
	if cancelWS != nil { cancelWS() }
	var wsCtx context.Context
	wsCtx, cancelWS = context.WithCancel(appCtx)
	startChatWS(proxyURL.String())  // <-- выделенный чат-WS на proxyURL
	...
	wsRefreshRemote(wsCtx, proxyURL.String(), onRefresh, nil)  // onChatMessage = nil
}()
```
`switchToWebDAVTree` уже вызывается и для отправителя (через `updateLink` в режиме дерева,
`send.go:772`), и для получателя (через callback прокси, `send.go:802`) — этого достаточно
(прототип long-poll тоже жил внутри `switchToWebDAVTree`).

### 6. `send.go` — подавить дубль-открытие при тапе по дереву

`createWebDAVTree.OnSelected` (`send.go:641-643`) — открывать через общий гейт:
```go
time.AfterFunc(100*time.Millisecond, func() {
	openBrowser(fullURLStr)
})
```
(`openBrowser` в scope `sendTabItem`, сигнатуру `createWebDAVTree` менять не нужно.)

---

## Поведение

| Сценарий | Результат |
|---|---|
| Получатель шлёт первым → браузер не открывали | `wsChat` отправителя (локально к `proxyURL`) получает сообщение → `openBrowser(chatURL)` один раз |
| Отправитель шлёт первым → у получателя | `wsChat` получателя (`proxyURL` = forwarder → туннель) получает → автооткрытие |
| Тап по дереву (локальное открытие) затем сообщение | `browserOpened=true` → дубликата нет |
| Пересоздание дерева / новая сессия | `browserOpened` сброшен → снова одно открытие |

## Соответствие прототипу (long-poll)
- Обособленность от refresh ✅ (прототип — отдельная горутина; здесь — отдельный `wsChat`).
- Подключение на обеих сторонах ✅ (guard проходит для отправителя и получателя).
- Гейт «один раз» + сброс на сессию ✅ (`chatOpened` atomic → `browserOpened`).
- Точка запуска — внутри `switchToWebDAVTree` ✅ (подтверждает, что у отправителя она выполняется).
- Транспорт: WS вместо long-poll (цель `c136659`); endpoint `/api/messages/wait` (`http.go:352`) ещё на месте.
- Целевой URL: `proxyURL` ✅ — как в прототипе.

## Что НЕ трогаем
- `wsRefreshRemote` (тело + назначение — refresh дерева).
- `onRefresh`/`scRefresh`/`broadcastRefresh`/`onFileTreeRefresh` — оповещение о файлах.
- `handleChatWS`/`broadcastChatMessage`/`directory.html`.

## Проверка
1. `go build ./...` / `go vet ./...`.
2. Отправитель (хост, режим дерева) + получатель (через туннель).
3. B: получатель первым пишет в чат → у отправителя открывается браузер. В логе отправителя:
   `[ws-chat] connected to ws://...:/api/chat/ws` и `[ws-chat] auto-opening browser`.
4. A: отправитель первым пишет → у получателя открывается браузер.
5. Тап по корню дерева → затем входящее сообщение → дубль-открытия нет.
6. Регресс: refresh дерева файлов работает как прежде (`broadcastRefresh`, без эхо у хоста).
