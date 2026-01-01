package hdmap

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ernmw/omwpacker/cfg"
	"github.com/stretchr/testify/require"
)

func BenchmarkGeneration(b *testing.B) {
	openmwcfg := "/home/ern/tes3/config/openmw.cfg"
	env, err := cfg.Load(openmwcfg)
	require.NoError(b, err)

	var rootPath string
	for _, plugin := range env.Plugins {
		if strings.EqualFold(filepath.Base(plugin), "livelymap.omwaddon") {
			rootPath = filepath.Dir(filepath.Dir(plugin))
		}
	}
	b.Logf("root: %q", rootPath)

	for b.Loop() {
		require.NoError(b, DrawMaps(b.Context(), rootPath, env, 6, true))
	}
}
