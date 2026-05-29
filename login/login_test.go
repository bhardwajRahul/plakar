package login

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/PlakarKorp/plakar/appcontext"
)

func TestNewLoginFlow(t *testing.T) {
	ctx := appcontext.NewAppContext()
	flow, err := NewLoginFlow(ctx, false)
	if err != nil {
		t.Fatalf("NewLoginFlow err = %v", err)
	}
	if flow == nil {
		t.Fatal("NewLoginFlow returned nil flow")
	}
	if flow.appCtx != ctx {
		t.Fatal("flow.appCtx not set")
	}
	if flow.noSpawn != false {
		t.Fatal("flow.noSpawn not set")
	}

	flow2, _ := NewLoginFlow(ctx, true)
	if !flow2.noSpawn {
		t.Fatal("noSpawn=true not honored")
	}
}

func TestRunRejectsUnsupportedProvider(t *testing.T) {
	ctx := appcontext.NewAppContext()
	flow, _ := NewLoginFlow(ctx, true)
	_, err := flow.Run("twitter", nil)
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("err = %v, want 'unsupported provider...'", err)
	}
}

func TestRunUIRejectsUnsupportedProvider(t *testing.T) {
	ctx := appcontext.NewAppContext()
	flow, _ := NewLoginFlow(ctx, true)
	_, err := flow.RunUI("facebook", nil)
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("err = %v", err)
	}
}

func TestPollReturnsContextErrorWhenAlreadyCancelled(t *testing.T) {
	ctx := appcontext.NewAppContext()
	cancelErr := errors.New("user cancelled")
	ctx.Cancel(cancelErr)

	flow, _ := NewLoginFlow(ctx, true)
	progressCalled := false
	token, err := flow.Poll("poll-id", 5, time.Millisecond, func() { progressCalled = true })
	if token != "" {
		t.Fatalf("token = %q, want empty", token)
	}
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if progressCalled {
		t.Fatal("progress callback should not fire when context is already cancelled")
	}
}

func TestCloseIsNoOp(t *testing.T) {
	ctx := appcontext.NewAppContext()
	flow, _ := NewLoginFlow(ctx, false)
	if err := flow.Close(); err != nil {
		t.Fatalf("Close = %v, want nil", err)
	}
}
