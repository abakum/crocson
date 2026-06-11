package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	wh "github.com/psanford/wormhole-william/wormhole"
	"github.com/psanford/wormhole-william/wormhole/tunnel"
	log "github.com/schollz/logger"
)

const (
	// DefaultWormholeMailbox = "ws://relay.magic-wormhole.io:4000/v1"
	// DefaultWormholeTransit = "transit.magic-wormhole.io:4001"
	DefaultWormholeMailbox = ""
	DefaultWormholeTransit = ""
)

type tunnelInterface interface {
	Dial(ctx context.Context, addr string) (net.Conn, error)
	Forward(ctx context.Context, localAddr string) error
	Serve(ctx context.Context, localAddr string) error
	Close() error
}

var _ tunnelInterface = (*tunnel.Tunnel)(nil)

func isWormholeRelay(addr string) bool {
	return strings.HasPrefix(addr, "ws:") || strings.HasPrefix(addr, "wss:") || strings.Contains(addr, "/v")
}

func maskSecret(p string) string {
	if len(p) > 2 {
		return fmt.Sprintf("%c***%c", p[0], p[len(p)-1])
	}
	return "***"
}

func resolveTransit(pass string) string {
	if pass != "" && pass != DEFAULT_PASSPHRASE {
		log.Debugf("wormhole transit=%s", maskSecret(pass))
		return pass
	}
	log.Debugf("wormhole transit=%s default", DefaultWormholeTransit)
	return DefaultWormholeTransit
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
		log.Debugf("wormhole mailbox=%s", addr)
		return ensureMailboxURL(addr)
	}
	log.Debugf("wormhole mailbox=%s default", DefaultWormholeMailbox)
	return DefaultWormholeMailbox
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

type WormholeTunnel struct {
	tunnel tunnelInterface
	cancel context.CancelFunc
	ctx    context.Context
	wg     sync.WaitGroup
}

func (wt *WormholeTunnel) Close() error {
	wt.cancel()
	var err error
	if wt.tunnel != nil {
		err = wt.tunnel.Close()
	}
	wt.wg.Wait()
	return err
}

func startWormholeSender(parentCtx context.Context, secret, mailboxURL, transitAddr, _ string) (string, func() (tunnelInterface, error), *WormholeTunnel, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	whClient := wh.Client{
		RendezvousURL:       ensureMailboxURL(mailboxURL),
		TransitRelayAddress: transitAddr,
	}

	code, connect, err := whClient.PrepareTunnel(ctx, secret)
	if err != nil {
		cancel()
		return "", nil, nil, fmt.Errorf("wormhole prepare tunnel: %w", err)
	}

	return code, func() (tunnelInterface, error) {
			t, err := connect()
			if err != nil {
				return nil, err
			}
			return t, nil
		}, &WormholeTunnel{
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
		}
		wt.cancel()
	}()

	return wt, nil
}
