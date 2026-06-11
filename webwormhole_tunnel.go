package main

import (
	"context"
	"fmt"
	"strings"

	log "github.com/schollz/logger"
	ww "webwormhole.io/wormhole"
	wtunnel "webwormhole.io/wormhole/tunnel"
)

var _ tunnelInterface = (*wtunnel.Tunnel)(nil)

func isWebWormholeRelay(addr string) bool {
	return strings.HasPrefix(addr, "https:") || strings.HasPrefix(addr, "http:")
}

func resolveSignalServer(addr string) string {
	if addr == "https:" || addr == "https:/" || addr == "https://" ||
		addr == "http:" || addr == "http:/" || addr == "http://" {
		return ""
	}
	return addr
}

func startWebWormholeSender(parentCtx context.Context, signalServer, _ string) (string, func() (tunnelInterface, error), *WormholeTunnel, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	wwClient := ww.Client{SignalServer: resolveSignalServer(signalServer)}

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

	wwClient := ww.Client{SignalServer: resolveSignalServer(signalServer)}

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
