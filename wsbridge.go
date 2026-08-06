// wsbridge.go routes croc's relay traffic through the getcroc.com WebSocket bridge.
//
// croc's comm.NewConnection already has a proxy branch: when comm.Socks5Proxy is set it calls
// proxy.FromURL (golang.org/x/net/proxy), which dispatches by URL scheme through a registry
// (proxy.RegisterDialerType). We register a "wss" scheme dialer that, for each Dial, opens a
// WebSocket to wss://getcroc.com/ws?port=<port> and returns it as a net.Conn via
// nhooyr.io/websocket.NetConn. croc then runs its full protocol (PAKE/room/chunks) over it,
// unaware that the transport is WebSocket instead of TCP. No changes to the croc fork.
package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"
	"nhooyr.io/websocket"
)

// wsBridgeBase is the full target that the "wss:" shorthand in the socks5 field expands to.
// The port is appended per connection (?port=N) from the address croc asks us to dial.
const wsBridgeBase = "wss://getcroc.com/ws"

var wsBridgeRegisterOnce sync.Once

// registerWSBridgeDialer registers the "wss" scheme with golang.org/x/net/proxy so that croc's
// comm.NewConnection routes through the bridge whenever comm.Socks5Proxy is a wss:// URL.
// Idempotent.
func registerWSBridgeDialer() {
	wsBridgeRegisterOnce.Do(func() {
		proxy.RegisterDialerType("wss", func(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
			return &wsBridgeDialer{base: u}, nil
		})
	})
}

// wsBridgeDialer implements proxy.Dialer. Each Dial opens a WebSocket to <base>?port=<port>
// extracted from the croc-supplied address (host:port) and returns a stream net.Conn.
type wsBridgeDialer struct {
	base *url.URL
}

func (d *wsBridgeDialer) Dial(network, address string) (net.Conn, error) {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("wsbridge: parse address %q: %w", address, err)
	}
	if port == "" {
		return nil, fmt.Errorf("wsbridge: empty port in address %q", address)
	}

	u := *d.base
	// webrelay слушает на /ws; если в базе нет пути (напр. "wss://host" без /ws),
	// дефолтим /ws — конвенция croc webrelay/getcroc.
	if u.Path == "" || u.Path == "/" {
		u.Path = "/ws"
	}
	q := u.Query()
	q.Set("port", port)
	u.RawQuery = q.Encode()
	target := u.String()

	// Transient drops are common on RU ISPs (observed ~1/5 on Rostelecom); retry briefly.
	var conn *websocket.Conn
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		c, _, dialErr := websocket.Dial(ctx, target, nil)
		cancel()
		if dialErr == nil {
			conn = c
			lastErr = nil
			break
		}
		lastErr = dialErr
		if attempt < 2 {
			time.Sleep(time.Duration(200*(attempt+1)) * time.Millisecond)
		}
	}
	if conn == nil {
		return nil, fmt.Errorf("wsbridge: dial %s: %w", target, lastErr)
	}

	streamCtx, cancelStream := context.WithCancel(context.Background())
	nc := websocket.NetConn(streamCtx, conn, websocket.MessageBinary)
	return &bridgeConn{Conn: nc, cancel: cancelStream}, nil
}

// bridgeConn wraps the NetConn so Close cancels the stream context (avoiding a leak).
// The underlying WebSocket is closed by NetConn.Close itself (netconn.go), so we must NOT close
// it a second time here — that produced "failed to close WebSocket: use of closed network connection".
type bridgeConn struct {
	net.Conn
	cancel context.CancelFunc
}

func (c *bridgeConn) Close() error {
	c.cancel()
	return c.Conn.Close()
}
