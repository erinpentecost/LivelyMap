package heightmap

import (
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenMwCfg(t *testing.T) {
	mm := NewMapMaker()
	plugins, err := mm.OpenMWPlugins("testdata/openmw.cfg", false)
	require.NoError(t, err)
	require.NotEmpty(t, plugins)

	// absolute paths
	require.Contains(t, plugins, PluginEntry{
		Name: "ernburglary.omwaddon",
		Path: "/home/ern/workspace/ErnBurglary/ErnBurglary.omwaddon",
	})
	require.Contains(t, plugins, PluginEntry{
		Name: "ernradianttheft.omwaddon",
		Path: "/home/ern/workspace/ErnRadiantTheft/ErnRadiantTheft.omwaddon",
	})
	// relative path
	expected, err := filepath.Abs("testdata/relativeExample.omwaddon")
	require.NoError(t, err)
	require.Contains(t, plugins, PluginEntry{
		Name: "relativeexample.omwaddon",
		Path: expected,
	})
}

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
	mm := NewMapMaker()
	plugins, err := mm.OpenMWPlugins(path, false)
	require.NoError(t, err)
	require.NotEmpty(t, plugins)

	x, err := mm.PluginsToBMP(plugins, "testdata/bmps")
	require.NoError(t, err)
	require.Greater(t, x, 10)
}
