package fileinfo

import (
	"io/fs"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewMockFileInfo(t *testing.T) {
	m := New()

	require.Equal(t, "test", m.Name())
	require.Equal(t, int64(100), m.Size())
	require.Equal(t, fs.FileMode(0644), m.Mode())
	require.False(t, m.IsDir())
	require.False(t, m.ModTime().IsZero())

	// Sys returns the underlying syscall.Stat_t pointer set by New.
	_, ok := m.Sys().(*syscall.Stat_t)
	require.True(t, ok, "Sys() should be a *syscall.Stat_t")
}

func TestMockFileInfoImplementsFSFileInfo(t *testing.T) {
	// MockFileInfo must satisfy the fs.FileInfo interface.
	var _ fs.FileInfo = New()
}
