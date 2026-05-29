package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	wh "github.com/psanford/wormhole-william/wormhole"
	"github.com/psanford/wormhole-william/wormhole/tunnel"
	log "github.com/schollz/logger"
)
// Режимы relay-address (в настройках "relay-address"):
//
// 1. Croc relay — адрес без ws:/wss: префикса
//    Пример: "croc.schollz.com"
//    Используется croc-протокол (TCP, свой rendezvous)
//
// 2. Wormhole relay (полный URL) — ws:/wss: с путём
//    Пример: "ws://relay.magic-wormhole.io:4000/v1"
//    Mailbox = адрес как есть
//    Transit = hostname:port+1 (вычисляется из mailbox)
//
// 3. Wormhole relay (короткий) — ws:/wss: без пути
//    Пример: "ws://" или "ws://my-server.com:4000"
//    Mailbox = ws://relay.magic-wormhole.io:4000/v1 (дефолтный)
//    Transit = transit.magic-wormhole.io:4001 (дефолтный)

const (
	// DefaultWormholeMailbox = "ws://relay.magic-wormhole.io:4000/v1"
	// DefaultWormholeTransit = "transit.magic-wormhole.io:4001"
	DefaultWormholeMailbox = ""
	DefaultWormholeTransit = ""
)

func isWormholeRelay(addr string) bool {
	return strings.HasPrefix(addr, "ws:") || strings.HasPrefix(addr, "wss:") || strings.Contains(addr, "/v")
}

func resolveMailboxURL(addr string) string {
	s := addr
	if s == "ws:" || s == "ws:/" || s == "ws://" {
		return DefaultWormholeMailbox
	}
	if strings.HasPrefix(s, "ws://") || strings.HasPrefix(s, "wss://") {
		s = s[strings.Index(s, "//")+2:]
	}
	if strings.Contains(s, "/v") {
		log.Debugf("wormhole mailbox=%s transit from mailbox", addr)
		return ensureMailboxURL(addr)
	}
	log.Debugf("wormhole mailbox=%s transit default", DefaultWormholeMailbox)
	return DefaultWormholeMailbox
}

func resolveTransitAddr(addr string) string {
	mailbox := resolveMailboxURL(addr)
	if mailbox == DefaultWormholeMailbox {
		log.Debugf("wormhole transit=%s default", DefaultWormholeTransit)
		return DefaultWormholeTransit
	}
	t := transitFromMailbox(addr)
	log.Debugf("wormhole transit=%s from mailbox", t)
	return t
}

func ensureMailboxURL(addr string) string {
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, "ws://") || strings.HasPrefix(addr, "wss://") {
		return addr
	}
	return "ws://" + addr
}

func transitFromMailbox(addr string) string {
	mailboxURL := ensureMailboxURL(addr)
	u, err := url.Parse(mailboxURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if strings.HasPrefix(host, "relay.") {
		host = "transit." + strings.TrimPrefix(host, "relay.")
	}
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "wss":
			port = "443"
		default:
			port = "80"
		}
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return net.JoinHostPort(host, port)
	}
	return net.JoinHostPort(host, strconv.Itoa(p+1))
}

type WormholeTunnel struct {
	tunnel *tunnel.Tunnel
	cancel context.CancelFunc
	ctx    context.Context
	wg     sync.WaitGroup
}

func (wt *WormholeTunnel) Close() error {
	wt.cancel()
	err := wt.tunnel.Close()
	wt.wg.Wait()
	return err
}

func startWormholeSender(parentCtx context.Context, secret, mailboxURL, transitAddr, webdavAddr string) (string, *WormholeTunnel, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	whClient := wh.Client{
		RendezvousURL:       ensureMailboxURL(mailboxURL),
		TransitRelayAddress: transitAddr,
	}

	code, t, err := whClient.CreateTunnel(ctx, secret)
	if err != nil {
		cancel()
		return "", nil, fmt.Errorf("wormhole create tunnel: %w", err)
	}

	return code, &WormholeTunnel{
		tunnel: t,
		cancel: cancel,
		ctx:    ctx,
	}, nil
}

func startWormholeReceiver(parentCtx context.Context, secret, mailboxURL, transitAddr, webdavAddr string) (*WormholeTunnel, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	whClient := wh.Client{
		RendezvousURL:       ensureMailboxURL(mailboxURL),
		TransitRelayAddress: transitAddr,
	}

	_, t, err := whClient.JoinTunnel(ctx, secret)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("wormhole join tunnel: %w", err)
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
			log.Errorf("wormhole tunnel forward: %v", err)
			wt.cancel()
		}
	}()

	return wt, nil
}
