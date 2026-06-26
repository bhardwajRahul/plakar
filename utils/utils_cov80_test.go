package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PlakarKorp/kloset/locate"
	"github.com/PlakarKorp/plakar/config"
	"github.com/stretchr/testify/require"
)

// clearHomeEnv removes every variable that os.UserHomeDir / the dir helpers
// consult so that GetCacheDir/GetConfigDir/GetDataDir fall into the
// os.UserHomeDir error branch.
func clearHomeEnvCov80(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"HOME", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME",
		"LocalAppData", "USERPROFILE", "HOMEDRIVE", "HOMEPATH",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestGetCacheDirHomeErrorCov80(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("home-error branch not reachable the same way on windows")
	}
	clearHomeEnvCov80(t)
	_, err := GetCacheDir("plakar")
	require.Error(t, err)
}

func TestGetConfigDirHomeErrorCov80(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("home-error branch not reachable the same way on windows")
	}
	clearHomeEnvCov80(t)
	_, err := GetConfigDir("plakar")
	require.Error(t, err)
}

func TestGetDataDirHomeErrorCov80(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("home-error branch not reachable the same way on windows")
	}
	clearHomeEnvCov80(t)
	_, err := GetDataDir("plakar")
	require.Error(t, err)
}

// shouldCheckUpdate returns false immediately because the test binary's VERSION
// is a "-devel" build; exercise that early-return branch explicitly.
func TestShouldCheckUpdateDevelEarlyReturnCov80(t *testing.T) {
	require.Contains(t, VERSION, "devel")
	dir := t.TempDir()
	require.False(t, shouldCheckUpdate(dir))
	// No cookie should have been created since we returned before touching disk.
	_, err := os.Stat(filepath.Join(dir, "last-update-check"))
	require.True(t, os.IsNotExist(err))
}

// LoadFallback should propagate the error from LoadOldConfigIfExists when the
// legacy plakar.yml is malformed.
func TestLoadFallbackOldConfigParseErrorCov80(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plakar.yml"), []byte("default-repo: [unterminated"), 0644))
	cl := newConfigHandler(dir)
	_, err := cl.LoadFallback()
	require.Error(t, err)
	require.Contains(t, err.Error(), "error reading file")
}

// Save should fail when MkdirAll cannot create the target directory.
func TestSaveMkdirErrorCov80(t *testing.T) {
	base := t.TempDir()
	filePath := filepath.Join(base, "regular")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0644))
	cl := newConfigHandler(filepath.Join(filePath, "sub"))
	err := cl.Save(config.NewConfig())
	require.Error(t, err)
}

// GetConf with a third-party prefix must rewrite each key with the prefix and
// add a synthetic location. Verify the rewrite path and that the "ignore"
// bookkeeping is exercised with multiple keys.
func TestGetConfThirdPartyMultiKeyCov80(t *testing.T) {
	in := "section1:\n  host: example.com\n  user: bob\n"
	out, err := GetConf(strings.NewReader(in), "myremote")
	require.NoError(t, err)
	sec := out["section1"]
	require.Equal(t, "myremote://", sec["location"])
	require.Equal(t, "example.com", sec["myremote_host"])
	require.Equal(t, "bob", sec["myremote_user"])
	// original keys must be gone
	_, hasHost := sec["host"]
	require.False(t, hasHost)
}

// GetConf must error when no location can be found and no third-party prefix is
// given.
func TestGetConfMissingLocationCov80(t *testing.T) {
	in := "section1:\n  host: example.com\n"
	_, err := GetConf(strings.NewReader(in), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing 'location' key")
}

// Set on a *time.Time field with an invalid value must surface an error
// (covers the setTime error branch via Set).
func TestPolicySetTimeInvalidCov80(t *testing.T) {
	c := &policiesConfig{Policies: map[string]*locate.LocateOptions{}}
	c.Add("p")
	err := c.Set("p", "before", "not-a-real-time")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid value")
}

// Set on a *bool field with a non-boolean value must error.
func TestPolicySetBoolInvalidCov80(t *testing.T) {
	c := &policiesConfig{Policies: map[string]*locate.LocateOptions{}}
	c.Add("p")
	err := c.Set("p", "latest", "notabool")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid value")
}

// Dump to a writer that fails should surface the encode error.
func TestPolicyDumpEncodeErrorCov80(t *testing.T) {
	c := &policiesConfig{Policies: map[string]*locate.LocateOptions{}}
	c.Add("p")
	err := c.Dump(failWriterCov80{}, "json", []string{"p"})
	require.Error(t, err)
}

type failWriterCov80 struct{}

func (failWriterCov80) Write(p []byte) (int, error) {
	return 0, errFailWriterCov80
}

var errFailWriterCov80 = &dumpWriteErr{}

type dumpWriteErr struct{}

func (*dumpWriteErr) Error() string { return "boom" }
