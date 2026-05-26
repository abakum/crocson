package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
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

const wormholeForwardAppID = "crocson.abakum.github.com/wormhole/forward"

func isWormholeRelay(addr string) bool {
	s := addr
	if strings.HasPrefix(s, "ws://") || strings.HasPrefix(s, "wss://") {
		s = s[strings.Index(s, "//")+2:]
	}
	return strings.Contains(s, "/")
}

func ensureMailboxURL(addr string) string {
	if strings.HasPrefix(addr, "ws://") || strings.HasPrefix(addr, "wss://") {
		return addr
	}
	return "ws://" + addr
}

func ensureWormholeCode(secret string) string {
	parts := strings.SplitN(secret, "-", 2)
	if len(parts) >= 2 {
		if _, err := strconv.Atoi(parts[0]); err == nil {
			return secret
		}
	}
	h := sha256.Sum256([]byte(secret))
	prefix := binary.BigEndian.Uint32(h[:4]) % 10000
	return fmt.Sprintf("%d-%s", prefix, secret)
}

func transitFromMailbox(addr string) string {
	mailboxURL := ensureMailboxURL(addr)
	u, err := url.Parse(mailboxURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
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
	tunnel   *tunnel.Tunnel
	cancel   context.CancelFunc
	ctx      context.Context
	wg       sync.WaitGroup
}

func (wt *WormholeTunnel) Close() error {
	wt.cancel()
	err := wt.tunnel.Close()
	wt.wg.Wait()
	return err
}

func startWormholeSender(parentCtx context.Context, secret, mailboxURL, transitAddr, webdavAddr string) (*WormholeTunnel, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	whClient := wh.Client{
		AppID:               wormholeForwardAppID,
		RendezvousURL:       ensureMailboxURL(mailboxURL),
		TransitRelayAddress: transitAddr,
	}

	code := ensureWormholeCode(secret)
	t, err := whClient.CreateTunnelWithCode(ctx, code)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("wormhole create tunnel: %w", err)
	}

	wt := &WormholeTunnel{
		tunnel: t,
		cancel: cancel,
		ctx:    ctx,
	}

	wt.wg.Add(1)
	go func() {
		defer wt.wg.Done()
		if err := t.Serve(ctx, webdavAddr); err != nil && ctx.Err() == nil {
			log.Errorf("wormhole tunnel serve: %v", err)
		}
	}()

	return wt, nil
}

func startWormholeReceiver(parentCtx context.Context, secret, mailboxURL, transitAddr, webdavAddr string) (*WormholeTunnel, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	whClient := wh.Client{
		AppID:               wormholeForwardAppID,
		RendezvousURL:       ensureMailboxURL(mailboxURL),
		TransitRelayAddress: transitAddr,
	}

	code := ensureWormholeCode(secret)
	t, err := whClient.JoinTunnel(ctx, code)
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
		if err := t.Forward(ctx, webdavAddr, webdavAddr); err != nil && ctx.Err() == nil {
			log.Errorf("wormhole tunnel forward: %v", err)
		}
	}()

	return wt, nil
}
