package dds

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecode1(t *testing.T) {
	raw, err := os.ReadFile("testdata/test_texture1.dds")
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	img, err := Decode(raw)
	require.NoError(t, err)
	require.NotNil(t, img)
	require.Equal(t, 16, img.Bounds().Dx())
	require.Equal(t, 16, img.Bounds().Dy())
}

func TestDecode2(t *testing.T) {
	raw, err := os.ReadFile("testdata/test_texture2.dds")
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	img, err := Decode(raw)
	require.NoError(t, err)
	require.NotNil(t, img)
	require.Equal(t, 1024, img.Bounds().Dx())
	require.Equal(t, 1024, img.Bounds().Dy())
}
