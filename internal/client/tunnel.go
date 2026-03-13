package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	singjson "github.com/sagernet/sing/common/json"

	"github.com/FrankFMY/burrow/internal/shared"
)

type Tunnel struct {
	instance *box.Box
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewTunnel(invite shared.InviteData) (*Tunnel, error) {
	registryCtx := include.Context(context.Background())
	ctx, cancel := context.WithCancel(registryCtx)

	opts, err := buildClientOptions(registryCtx, invite)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("build client config: %w", err)
	}

	instance, err := box.New(box.Options{
		Context: ctx,
		Options: opts,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create sing-box instance: %w", err)
	}

	return &Tunnel{instance: instance, ctx: ctx, cancel: cancel}, nil
}

func (t *Tunnel) Start() error {
	if err := t.instance.Start(); err != nil {
		return fmt.Errorf("start tunnel: %w", err)
	}
	slog.Info("tunnel started")
	return nil
}

func (t *Tunnel) Close() error {
	t.cancel()
	return t.instance.Close()
}

func (t *Tunnel) Wait() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	s := <-sig
	slog.Info("received signal", "signal", s)
}

func buildClientOptions(ctx context.Context, invite shared.InviteData) (option.Options, error) {
	configMap := map[string]any{
		"log": map[string]any{
			"level": "info",
		},
		"dns": map[string]any{
			"servers": []map[string]any{
				{
					"tag":     "remote-doh",
					"address": "https://1.1.1.1/dns-query",
					"detour":  "vless-out",
				},
				{
					"tag":     "local-dns",
					"address": "223.5.5.5",
					"detour":  "direct-out",
				},
			},
			"rules": []map[string]any{
				{
					"outbound": []string{"any"},
					"server":   "remote-doh",
				},
			},
		},
		"inbounds": []any{
			map[string]any{
				"type":        "mixed",
				"tag":         "mixed-in",
				"listen":      "127.0.0.1",
				"listen_port": 1080,
			},
		},
		"outbounds": []any{
			map[string]any{
				"type":        "vless",
				"tag":         "vless-out",
				"server":      invite.Server,
				"server_port": invite.Port,
				"uuid":        invite.Token,
				"tls": map[string]any{
					"enabled":     true,
					"server_name": invite.SNI,
					"utls": map[string]any{
						"enabled":     true,
						"fingerprint": "chrome",
					},
					"reality": map[string]any{
						"enabled":    true,
						"public_key": invite.PublicKey,
						"short_id":   invite.ShortID,
					},
				},
			},
			map[string]any{
				"type": "direct",
				"tag":  "direct-out",
			},
		},
		"route": map[string]any{
			"rules": []map[string]any{
				{
					"action":   "hijack-dns",
					"protocol": []string{"dns"},
				},
			},
			"final": "vless-out",
		},
	}

	b, err := json.Marshal(configMap)
	if err != nil {
		return option.Options{}, fmt.Errorf("marshal config: %w", err)
	}

	opts, err := singjson.UnmarshalExtendedContext[option.Options](ctx, b)
	if err != nil {
		return option.Options{}, fmt.Errorf("parse sing-box config: %w", err)
	}

	return opts, nil
}
