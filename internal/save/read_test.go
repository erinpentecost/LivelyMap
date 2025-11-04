package save

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenMwCfg(t *testing.T) {

	section, err := ExtractPlayerStorageSection("testdata/livelymap.omwsave", "LivelyMap")
	require.NoError(t, err)
	require.NotNil(t, section)
}
