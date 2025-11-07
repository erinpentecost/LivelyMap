package savefile

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenMwCfg(t *testing.T) {
	paths, err := ExtractSaveData("testdata/livelymap.omwsave")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
}
