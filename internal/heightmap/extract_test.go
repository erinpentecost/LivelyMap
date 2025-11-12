package heightmap

import (
	_ "embed"
	"os"
	"testing"

	"github.com/ernmw/omwpacker/cfg"
	"github.com/stretchr/testify/require"
)

func cfgPath(t *testing.T) string {
	t.Helper()
	// Get a real openmw.cfg
	cfgPath := os.Getenv("OMW_CONFIG")
	if len(cfgPath) == 0 {
		cfgPath = "/home/ern/tes3/config/openmw.cfg"
	}
	fs, err := os.Stat(cfgPath)
	require.NoError(t, err)
	require.False(t, fs.IsDir())
	return cfgPath
}

func TestGeneration(t *testing.T) {
	// Read plugins from a real install.
	path := cfgPath(t)
	plugins, _, err := cfg.OpenMWPlugins(path)
	require.NoError(t, err)
	require.NotEmpty(t, plugins)

	x, err := PluginsToBMP(plugins, "testdata/bmps")
	require.NoError(t, err)
	require.Greater(t, x, 10)
}
