package savefile

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenMwCfg(t *testing.T) {
	raw, err := ExtractSaveData("testdata/livelymap.omwsave")
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	paths, err := Unmarshal(raw)
	require.NoError(t, err)
	require.NotEmpty(t, paths)
}
