package luabin

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenMwCfg(t *testing.T) {
	store, err := LoadStorageFile("testdata/player_storage.bin")
	require.NoError(t, err)
	section, ok := store.GetSection("LivelyMap")
	require.Truef(t, ok, "section not found")

	for key, raw := range section {
		val, err := Deserialize(raw)
		require.NoError(t, err, "error decoding %q", key)
		t.Logf("%s = %#v\n", key, val)
	}
}
