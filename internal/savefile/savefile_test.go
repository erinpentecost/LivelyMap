package savefile

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const saveFile = "testdata/livelymap.omwsave"
const backupFile = "testdata/livelymap.bak"

func TestOpenMwCfg(t *testing.T) {
	t.Cleanup(func() {
		// restore backup file
		if _, err := os.Stat(backupFile); err != nil {
			t.FailNow()
			return
		}
		src, err := os.Create(saveFile)
		require.NoError(t, err)
		backup, err := os.Open(backupFile)
		require.NoError(t, err)
		_, err = io.Copy(src, backup)
		require.NoError(t, err)
	})

	initialSaveData, err := os.ReadFile(saveFile)
	require.NoError(t, err)

	saveData, err := ExtractData(saveFile)
	require.NoError(t, err)
	require.NotNil(t, saveData)
	require.NotEmpty(t, saveData.Paths)

	// did the backup work?
	backupSaveData, err := os.ReadFile(backupFile)
	require.NoError(t, err)
	require.Equal(t, initialSaveData, backupSaveData)

	// did we edit the save file?
	newSaveData, err := os.ReadFile(saveFile)
	require.NoError(t, err)
	require.NotEqual(t, initialSaveData, newSaveData)

	// did the extract work?
	require.True(t, !bytes.Contains(newSaveData, []byte(magic_prefix)))
}
