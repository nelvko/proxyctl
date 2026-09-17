package app

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/nelvko/proxyctl/internal/kernel"
)

func TestWaitReachableCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitReachable(ctx, kernel.Endpoints{HTTP: "http://127.0.0.1:1"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestStartProxyReadiness(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	a, k, _ := switchingApp(t, false)
	k.endpoints = kernel.Endpoints{SOCKS: "socks5h://" + listener.Addr().String()}
	a.Subscriptions = nil
	got, err := a.StartProxy(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got != k.endpoints || k.starts != 1 || a.Subscriptions != nil {
		t.Fatalf("unexpected startup: %+v starts=%d", got, k.starts)
	}
	if _, err := a.StartProxy(t.Context()); err != nil {
		t.Fatal(err)
	}
	if k.starts != 1 {
		t.Fatal("already-running service was started again")
	}
}

func TestStartProxyDoesNotStartAfterResolutionOrValidationFailure(t *testing.T) {
	for _, kind := range []string{"endpoints", "validation", "cancellation"} {
		t.Run(kind, func(t *testing.T) {
			a, k, _ := switchingApp(t, false)
			expected := errors.New(kind)
			ctx := t.Context()
			switch kind {
			case "endpoints":
				k.endpointErr = expected
			case "validation":
				k.validationErr = expected
			case "cancellation":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				expected = context.Canceled
			}
			if _, err := a.StartProxy(ctx); !errors.Is(err, expected) {
				t.Fatalf("got %v, want %v", err, expected)
			}
			if k.starts != 0 {
				t.Fatal("started despite failure")
			}
		})
	}
}
