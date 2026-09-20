package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// import of a config that contains no usable sections must error with "no
// valid ...s found".
// ---------- store show -ini format over a populated store ----------

func TestDispatchShowINIFormatCov80(t *testing.T) {
	ctx, bufOut, _ := newCtx(t)
	require.NoError(t, dispatchSubcommand(ctx, "store", "add",
		[]string{"r", "fs://" + t.TempDir(), "extra=val"}))
	bufOut.Reset()
	require.NoError(t, dispatchSubcommand(ctx, "store", "show", []string{"-ini", "r"}))
	out := bufOut.String()
	require.Contains(t, out, "[r]")
	require.Contains(t, out, "extra")
}
