package heightmap

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenMwCfg(t *testing.T) {
	mm := NewMapMaker()
	plugins, err := mm.OpenMWPlugins("testdata/openmw.cfg", false)
	require.NoError(t, err)
	require.NotEmpty(t, plugins)

	// absolute paths
	require.Contains(t, plugins, "ernburglary.omwaddon")
	require.Equal(t, plugins["ernburglary.omwaddon"], "/home/ern/workspace/ErnBurglary/ErnBurglary.omwaddon")
	require.Contains(t, plugins, "ernradianttheft.omwaddon")
	require.Equal(t, plugins["ernradianttheft.omwaddon"], "/home/ern/workspace/ErnRadiantTheft/ErnRadiantTheft.omwaddon")
	// relative path
	require.Contains(t, plugins, "relativeexample.omwaddon")
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

	// Check for Tamriel Rebuilt
	require.Contains(t, plugins, "tr_mainland.esm")
	expectedTR := filepath.Join("00 Core", "TR_Mainland.esm")
	require.True(t,
		strings.HasSuffix(plugins["tr_mainland.esm"], expectedTR))

	// Check for Bloodmoon
	require.Contains(t, plugins, "bloodmoon.esm")
	expectedBM := filepath.Join("Data Files", "Bloodmoon.esm")
	require.True(t,
		strings.HasSuffix(plugins["bloodmoon.esm"], expectedBM))

	x, err := mm.PluginsToBMP(plugins, "testdata/bmps")
	require.NoError(t, err)
	require.Greater(t, x, 10)
}
