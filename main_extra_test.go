package main

import (
	"testing"

	"github.com/PlakarKorp/kloset/connectors/storage"
	"github.com/PlakarKorp/kloset/encryption"
	"github.com/stretchr/testify/require"
)

func TestGetPassphraseFromEnv_PassphraseCmd(t *testing.T) {
	ctx := newTestCtx(t)
	// passphrase_cmd runs a shell command whose stdout is the passphrase.
	params := map[string]string{"passphrase_cmd": "echo hunter2"}
	pass, err := getPassphraseFromEnv(ctx, params)
	require.NoError(t, err)
	require.Equal(t, "hunter2", pass)
	// the key is consumed from the params map
	_, ok := params["passphrase_cmd"]
	require.False(t, ok)
}

func TestSetupEncryption_KeyFromFileCorrect(t *testing.T) {
	ctx := newTestCtx(t)
	cfg := storage.NewConfiguration()
	require.NotNil(t, cfg.Encryption)

	passphrase := "correct horse battery staple"
	key, err := encryption.DeriveKey(cfg.Encryption.KDFParams, []byte(passphrase))
	require.NoError(t, err)
	canary, err := encryption.DeriveCanary(cfg.Encryption, key)
	require.NoError(t, err)
	cfg.Encryption.Canary = canary

	ctx.KeyFromFile = passphrase
	require.NoError(t, setupEncryption(ctx, cfg))
}

func TestSetupEncryption_KeyFromFileWrong(t *testing.T) {
	ctx := newTestCtx(t)
	cfg := storage.NewConfiguration()

	// Set a canary derived from the right passphrase...
	key, err := encryption.DeriveKey(cfg.Encryption.KDFParams, []byte("right"))
	require.NoError(t, err)
	canary, err := encryption.DeriveCanary(cfg.Encryption, key)
	require.NoError(t, err)
	cfg.Encryption.Canary = canary

	// ...but unlock with the wrong one: setupEncryption fails with ErrCantUnlock.
	ctx.KeyFromFile = "wrong"
	err = setupEncryption(ctx, cfg)
	require.ErrorIs(t, err, ErrCantUnlock)
}

