package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------- GetPassphraseFromCommand ----------

func TestGetPassphraseFromCommandSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh syntax")
	}
	pass, err := GetPassphraseFromCommand("printf 'sekret'")
	require.NoError(t, err)
	require.Equal(t, "sekret", pass)
}

func TestGetPassphraseFromCommandSingleLineEcho(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh syntax")
	}
	pass, err := GetPassphraseFromCommand("echo hunter2")
	require.NoError(t, err)
	require.Equal(t, "hunter2", pass)
}

func TestGetPassphraseFromCommandMultilineError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh syntax")
	}
	// Two output lines must be rejected.
	_, err := GetPassphraseFromCommand("printf 'a\\nb\\n'")
	require.Error(t, err)
	require.Contains(t, err.Error(), "too many lines")
}

func TestGetPassphraseFromCommandEmptyOutputError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh syntax")
	}
	// No output -> zero lines -> "too many lines" (lines != 1).
	_, err := GetPassphraseFromCommand("true")
	require.Error(t, err)
}

func TestGetPassphraseFromCommandFailingCmd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh syntax")
	}
	// A command that prints one line but exits non-zero -> Wait() errors.
	_, err := GetPassphraseFromCommand("echo oops; exit 3")
	require.Error(t, err)
}

// ---------- LoadOldConfigIfExists additional paths ----------

func TestLoadOldConfigIfExistsMissingDistinct(t *testing.T) {
	cfg, err := LoadOldConfigIfExists(filepath.Join(t.TempDir(), "nope.yml"))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Empty(t, cfg.Repositories)
}

func TestLoadOldConfigIfExistsDecodeErrorDistinct(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plakar.yml")
	require.NoError(t, os.WriteFile(path, []byte("default-repo: [unterminated"), 0o600))
	_, err := LoadOldConfigIfExists(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse old config file")
}

// ---------- config.go load() previous-format fallback for sources/destinations ----------

func TestLoadConfigPreviousFormatSourcesAndDestinations(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	// No version field -> falls through to the previous (top-level map) format.
	mustWrite("sources.yml", "s1:\n  location: fs:///s1\n")
	mustWrite("destinations.yml", "d1:\n  location: fs:///d1\n")
	mustWrite("stores.yml", "version: v1.0.0\nstores:\n  r1:\n    location: fs:///r1\n")

	cfg, err := LoadConfig(dir)
	require.NoError(t, err)
	require.Equal(t, "fs:///s1", cfg.Sources["s1"]["location"])
	require.Equal(t, "fs:///d1", cfg.Destinations["d1"]["location"])
}

// ---------- config.go load() parse error path ----------

func TestLoadConfigSourcesParseError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sources.yml"),
		[]byte("not: [valid: yaml"), 0o600))
	_, err := LoadConfig(dir)
	require.Error(t, err)
}

// ---------- SaveConfig error path: ConfigDir is a regular file ----------

func TestSaveConfigMkdirFails(t *testing.T) {
	// Point ConfigDir at a path whose parent is a regular file -> MkdirAll fails.
	f := filepath.Join(t.TempDir(), "afile")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))

	cfg, err := LoadOldConfigIfExists(filepath.Join(t.TempDir(), "missing.yml"))
	require.NoError(t, err)

	err = SaveConfig(filepath.Join(f, "subdir"), cfg)
	require.Error(t, err)
}

// ---------- LoadConfig empty-file branch in load() ----------

func TestLoadConfigEmptySourcesFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, body string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	// Empty sources.yml (size 0) returns nil early; the rest are valid.
	mustWrite("sources.yml", "")
	mustWrite("destinations.yml", "version: v1.0.0\ndestinations:\n  d:\n    location: fs:///d\n")
	mustWrite("stores.yml", "version: v1.0.0\nstores:\n  r:\n    location: fs:///r\n")

	cfg, err := LoadConfig(dir)
	require.NoError(t, err)
	require.Equal(t, "fs:///r", cfg.Repositories["r"]["location"])
}
