package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateEmail(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, err := ValidateEmail("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty")
	})

	t.Run("valid plain", func(t *testing.T) {
		addr, err := ValidateEmail("user@example.com")
		require.NoError(t, err)
		require.Equal(t, "user@example.com", addr)
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := ValidateEmail("not-an-email")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid email address")
	})

	t.Run("rejects display-name form", func(t *testing.T) {
		// mail.ParseAddress accepts "Alice <alice@example.com>", but
		// ValidateEmail's strict check requires the input to equal the
		// parsed Address — so this must be rejected.
		_, err := ValidateEmail("Alice <alice@example.com>")
		require.Error(t, err)
	})
}

func TestGetDataDir_FromEnv(t *testing.T) {
	tempDir := t.TempDir()
	envVar := "XDG_DATA_HOME"
	if runtime.GOOS == "windows" {
		envVar = "LocalAppData"
	}
	t.Setenv(envVar, tempDir)

	dataDir, err := GetDataDir("testapp")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(tempDir, "testapp"), dataDir)
	require.True(t, filepath.IsAbs(dataDir))
}

func TestGetDataDir_FallbackToHome(t *testing.T) {
	envVar := "XDG_DATA_HOME"
	if runtime.GOOS == "windows" {
		envVar = "LocalAppData"
	}
	t.Setenv(envVar, "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", homeDir)
	}

	dataDir, err := GetDataDir("testapp")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(dataDir, homeDir),
		"dataDir %q should be under fallback home %q", dataDir, homeDir)
	require.Contains(t, dataDir, "testapp")
}

func TestGetCacheDir_FallbackToHome(t *testing.T) {
	envVar := "XDG_CACHE_HOME"
	if runtime.GOOS == "windows" {
		envVar = "LocalAppData"
	}
	t.Setenv(envVar, "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", homeDir)
	}

	cacheDir, err := GetCacheDir("testapp")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(cacheDir, homeDir),
		"cacheDir %q should be under fallback home %q", cacheDir, homeDir)
	require.Contains(t, cacheDir, "testapp")
}

func TestGetConfigDir_FallbackToHome(t *testing.T) {
	envVar := "XDG_CONFIG_HOME"
	if runtime.GOOS == "windows" {
		envVar = "LocalAppData"
	}
	t.Setenv(envVar, "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", homeDir)
	}

	configDir, err := GetConfigDir("testapp")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(configDir, homeDir),
		"configDir %q should be under fallback home %q", configDir, homeDir)
	require.Contains(t, configDir, "testapp")
}

func TestShouldCheckUpdate(t *testing.T) {
	cacheDir := t.TempDir()

	// `shouldCheckUpdate` short-circuits to false when VERSION contains "devel".
	// In a non-CI dev build that's the case, so we detect that and assert the
	// devel short-circuit; otherwise we drive the time-based path.
	if strings.Contains(VERSION, "devel") {
		require.False(t, shouldCheckUpdate(cacheDir))
		return
	}

	// First call: no cookie yet -> should return true and create the cookie file.
	require.True(t, shouldCheckUpdate(cacheDir))

	cookie := filepath.Join(cacheDir, "last-update-check")
	_, err := os.Stat(cookie)
	require.NoError(t, err, "shouldCheckUpdate must create the cookie file")

	// Second call within 24h: returns false.
	require.False(t, shouldCheckUpdate(cacheDir))

	// Backdate the cookie past 24h and confirm it flips back to true.
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(cookie, old, old))
	require.True(t, shouldCheckUpdate(cacheDir))
}
