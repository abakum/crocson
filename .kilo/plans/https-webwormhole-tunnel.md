# План: WebWormhole туннель при relay `https://`

## Цель

Когда relay-адрес в режиме WebDAV начинается на `https:` (или `http:`), использовать **webwormhole** туннель (WebRTC DataChannels).

| Relay-адрес | Туннель | Транспорт |
|---|---|---|
| `ws://...` / `wss://...` | wormhole-william (существующий) | transit TCP |
| `https://...` / `http://...` | webwormhole (новый) | WebRTC DataChannels |
| `host:port` | croc relay | croc TCP |

## Поток webwormhole

**Отправитель** (не вводит секрет, просто жмёт Send):
1. Пропускаем `entry.Validate()` для `https:` relay
2. `ww.Client.PrepareTunnel(ctx, "")` → локально генерирует 2-байтный PAKE-пароль, быстро получает slot от сервера → возвращает код (например `"7-acorn-july"`) + connect-функцию
3. Код отображается и копируется в буфер (`code != secret` → автокопирование через существующую логику `send.go:909`)
4. `connect()` блокируется до подключения получателя → возвращает `*tunnel.Tunnel`
5. `tunnel.Serve(ctx, webdavAddr)` — принимает соединения через туннель

**Получатель** (вводит код отправителя):
1. Вводит webwormhole код `"7-acorn-july"` → проходит `entry.Validate()` (>5 символов)
2. `ww.Client.JoinTunnel(ctx, code)` → `wordlist.Decode(code)` → `JoinTunnelCtx(ctx, slot, pass, signalServer)` → PAKE → WebRTC
3. `tunnel.Forward(ctx, webdavAddr)` — локальный TCP listener, проксирует через туннель

## tunnel.Tunnel API

Оба пакета (`github.com/psanford/wormhole-william/wormhole/tunnel` и `webwormhole.io/wormhole/tunnel`) предоставляют `*tunnel.Tunnel` с идентичными методами (разные Go-типы):

- `Serve(ctx, localAddr) error`
- `Forward(ctx, localAddr) error`
- `Close() error`
- `Dial(ctx, addr) (net.Conn, error)`

## План

### Шаг 1: Интерфейс туннеля в `wormhole_tunnel.go`

```go
type tunnelInterface interface {
    Dial(ctx context.Context, addr string) (net.Conn, error)
    Forward(ctx context.Context, localAddr string) error
    Serve(ctx context.Context, localAddr string) error
    Close() error
}

type WormholeTunnel struct {
    tunnel tunnelInterface
    cancel context.CancelFunc
    ctx    context.Context
    wg     sync.WaitGroup
}
```

### Шаг 2: Определение типа relay

```go
func isWebWormholeRelay(addr string) bool {
    return strings.HasPrefix(addr, "https:") || strings.HasPrefix(addr, "http:")
}
```

### Шаг 3: Функции webwormhole в `wormhole_tunnel.go`

Импорты:

```go
import (
    ww "webwormhole.io/wormhole"
)
```

```go
func startWebWormholeSender(parentCtx context.Context, signalServer, _ string) (string, func() (tunnelInterface, error), *WormholeTunnel, error) {
    ctx, cancel := context.WithCancel(parentCtx)

    wwClient := ww.Client{SignalServer: signalServer}

    code, connect, err := wwClient.PrepareTunnel(ctx, "")
    if err != nil {
        cancel()
        return "", nil, nil, fmt.Errorf("webwormhole prepare tunnel: %w", err)
    }

    return code, func() (tunnelInterface, error) {
        t, err := connect()
        if err != nil {
            return nil, err
        }
        return t, nil
    }, &WormholeTunnel{cancel: cancel, ctx: ctx}, nil
}

func startWebWormholeReceiver(parentCtx context.Context, code, signalServer, _ string, webdavAddr string) (*WormholeTunnel, error) {
    ctx, cancel := context.WithCancel(parentCtx)

    wwClient := ww.Client{SignalServer: signalServer}

    _, t, err := wwClient.JoinTunnel(ctx, code)
    if err != nil {
        cancel()
        return nil, fmt.Errorf("webwormhole join tunnel: %w", err)
    }

    wt := &WormholeTunnel{
        tunnel: t,
        cancel: cancel,
        ctx:    ctx,
    }

    wt.wg.Add(1)
    go func() {
        defer wt.wg.Done()
        if err := t.Forward(ctx, webdavAddr); err != nil && ctx.Err() == nil {
            log.Errorf("webwormhole tunnel forward: %v", err)
        }
        wt.cancel()
    }()

    return wt, nil
}
```

### Шаг 4: Обновить `startWormholeSender` / `startWormholeReceiver`

Изменить тип возврата `connect()` на `func() (tunnelInterface, error)`:

```go
func startWormholeSender(...) (string, func() (tunnelInterface, error), *WormholeTunnel, error) {
    ...
    return code, func() (tunnelInterface, error) {
        t, err := connect()
        return t, err
    }, ...
}
```

### Шаг 5: `send.go` — добавить ветку webwormhole (~стр. 830)

Ключевое отличие: **пропускаем `entry.Validate()`** и **не требуем `seady()`** (WebDAV режим не требует файлов).

```go
mainButton = widget.NewButtonWithIcon(lp("Send"), theme.MailSendIcon(), func() {
    fyne.Do(func() { topline.SetText("") })

    relayAddr := a.Preferences().String("relay-address")

    // === WEBWORMHOLE ПУТЬ (https: relay) ===
    // Отправитель не вводит секрет — код генерируется webwormhole
    if isWebWormholeRelay(relayAddr) {
        webdavAddr := davServer.addr
        if webdavAddr == "" || !davServer.IsActive() {
            fyne.Do(func() {
                topline.SetText(lp("Use WebDAV for webwormhole relay"))
                NewToast(w, lp("Use WebDAV for webwormhole relay")).Show()
            })
            return
        }
        signalServer := relayAddr
        showCancel()
        fyne.Do(func() {
            allEnabled(false, cosED...)
            if treeButton.Icon == theme.VisibilityOffIcon() {
                allEnabled(true, cosDAV...)
            }
            if totpCheck.Checked {
                totpProg.Hide()
            }
            topline.SetText(lp("Have them press the Download now"))
        })
        go func() {
            var wormholeCtx context.Context
            wormholeCtx, wormholeCancel = context.WithCancel(appCtx)

            defer func() {
                wormholeCancel = nil
                select {
                case <-cancelChan:
                default:
                    close(cancelChan)
                }
                davServer.DisableTCPForwarding()
                caffeinate(-1)
                fyne.Do(func() {
                    cancelButton.Hide()
                    mainButton.Show()
                    davServer.SetLocal(false)
                    if treeButton.Icon == theme.VisibilityIcon() {
                        davControl.Hide()
                    }
                    allShow(false, cosSH...)
                    allEnabled(true, cosED...)
                    if totpCheck.Checked {
                        totpProg.Show()
                    }
                    reload()
                    showPage()
                    log.Warnf("NumGoroutine %d", runtime.NumGoroutine())
                })
            }()

            caffeinate(1)

            code, connectFn, wt, err := startWebWormholeSender(wormholeCtx, signalServer, "")
            if err != nil {
                log.Errorf("webwormhole sender: %v", err)
                fyne.Do(func() {
                    if wormholeCtx.Err() != nil {
                        topline.SetText(lp("Send cancelled."))
                    } else {
                        topline.SetText(err.Error())
                    }
                })
                return
            }
            defer wt.Close()

            // Код сгенерирован — показать и скопировать
            fyne.Do(func() {
                a.Preferences().SetString("secret", code)
                setClipboard(a)
                NewToast(w, code).Show()
            })

            t, err := connectFn()
            if err != nil {
                log.Errorf("webwormhole connect: %v", err)
                fyne.Do(func() {
                    if wormholeCtx.Err() != nil {
                        topline.SetText(lp("Send cancelled."))
                    } else {
                        topline.SetText(err.Error())
                    }
                })
                return
            }
            wt.tunnel = t
            log.Debugf("[webwormhole] calling Serve localAddr=%s", webdavAddr)
            if err := wt.tunnel.Serve(wormholeCtx, webdavAddr); err != nil && wormholeCtx.Err() == nil {
                log.Errorf("webwormhole tunnel serve: %v", err)
            }
        }()
        return
    }

    // === Существующая валидация (croc + wormhole-william) ===
    if entry.Validate() != nil {
        // ... существующий код ...
    }
    // ...
})
```

### Шаг 6: `recv.go` — добавить ветку webwormhole (~стр. 630)

Получатель вводит webwormhole-код (валидация >5 символов проходит).

```go
// В обработчике mainButton:
if entry.Validate() != nil {
    // ... существующий код ...
    return
}

secret := entry.Text
// ... totp ...

relayAddr := a.Preferences().String("relay-address")
if isWormholeRelay(relayAddr) {
    // ... существующий wormhole-william код ...
    return
}
if isWebWormholeRelay(relayAddr) {
    webdavAddr := davServer.addr
    if webdavAddr == "" || !davServer.IsActive() {
        fyne.Do(func() {
            topline.SetText(lp("Use WebDAV for webwormhole relay"))
            NewToast(w, lp("Use WebDAV for webwormhole relay")).Show()
        })
        return
    }
    signalServer := relayAddr
    showCancel()
    allEnabled(false, cosED...)
    if davServer.IsActive() {
        allEnabled(true, cosDAV...)
    }
    if totpCheck.Checked {
        totpProg.Hide()
    }
    go func() {
        davServer.Stop()
        var wormholeCtx context.Context
        wormholeCtx, wormholeCancel = context.WithCancel(appCtx)

        defer func() {
            wormholeCancel = nil
            select {
            case <-cancelChan:
            default:
                close(cancelChan)
            }
            davServer.NotifyProxyState(false)
            davServer.DisableTCPForwarding()
            caffeinate(-1)
            fyne.Do(func() {
                cancelButton.Hide()
                mainButton.Show()
                allShow(false, cosSH...)
                allEnabled(true, cosED...)
                if totpCheck.Checked {
                    totpProg.Show()
                }
                reload()
                showPage()
                log.Warnf("NumGoroutine %d", runtime.NumGoroutine())
            })
            davServer.Start(webdavAddr, join(), false)
        }()

        caffeinate(1)

        wt, err := startWebWormholeReceiver(wormholeCtx, secret, signalServer, "", webdavAddr)
        if err != nil {
            log.Errorf("webwormhole receiver: %v", err)
            fyne.Do(func() {
                if wormholeCtx.Err() != nil {
                    topline.SetText(lp("Receive cancelled."))
                } else {
                    topline.SetText(err.Error())
                }
            })
            return
        }
        defer wt.Close()
        fyne.Do(func() {
            topline.SetText(lp("Connected via webwormhole"))
            davServer.SetLocal(true)
        })
        davServer.NotifyProxyState(true)
        select {
        case <-appCtx.Done():
        case <-cancelChan:
        case <-wt.ctx.Done():
        }
    }()
    return
}
```

### Шаг 7: `go.mod` — добавить webwormhole.io

```
replace webwormhole.io => ../webwormhole
```

```
go mod tidy
```

## Файлы

| Файл | Действие | Описание |
|---|---|---|
| `wormhole_tunnel.go` | Изменить | `tunnelInterface`, `isWebWormholeRelay`, `startWebWormholeSender/Receiver`, обновить возврат `startWormholeSender/Receiver` |
| `send.go` | Изменить | Ветка `isWebWormholeRelay` ДО `entry.Validate()`, без требования ввода секрета |
| `recv.go` | Изменить | Ветка `isWebWormholeRelay` после `isWormholeRelay`, получатель вводит webwormhole-код |
| `go.mod` | Изменить | Добавить `webwormhole.io` |

## Замечания

1. **Отправитель не вводит секрет**: `PrepareTunnel("")` генерирует код мгновенно (один round-trip к серверу за slot). Код копируется в буфер.

2. **Получатель вводит код**: webwormhole-код (например `"7-acorn-july"`) проходит стандартную валидацию (>5 символов).

3. **Transit**: webwormhole использует WebRTC (peer-to-peer с STUN/TURN fallback). Transit relay не нужен.

4. **Signalling**: URL relay (`https://...`) → `SignalServer` → webwormhole конвертирует `https://` → `wss://` для WebSocket.

5. **Шифрование**: CPACE PAKE + DTLS (WebRTC).

6. **Ветки `ws:` / `wss:`**: не меняются — wormhole-william.
