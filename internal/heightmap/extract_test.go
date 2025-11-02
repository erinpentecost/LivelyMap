package heightmap

import (
	_ "embed"
	"testing"
)

//go:embed testdata/openmw.cfg
var testOpenMWcfg []byte

func TestOpenMwCfg(t *testing.T) {
	mm := NewMapMaker()
	plugins, err := mm.OpenMWPlugins("testdata/openmw.cfg", false)
	require.NoError(t, err)
	require.NotEmpty(t, plugins)
}
