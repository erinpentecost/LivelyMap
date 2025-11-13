package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erinpentecost/LivelyMap/internal/hdmap"
	"github.com/ernmw/omwpacker/cfg"
)

const plugin_name = "livelymap.omwaddon"

func sync(path string) error {
	ctx := context.Background()

	plugins, _, err := cfg.OpenMWPlugins(path)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}

	// find path to dump map to
	var targetDir string
	for _, plugin := range plugins {
		print(plugin + "\n")

		if strings.EqualFold(filepath.Base(plugin), plugin_name) {
			targetDir = filepath.Join(filepath.Dir(plugin), "scripts", "LivelyMap", "dump")
		}
	}
	if tdir_, err := os.Stat(targetDir); err != nil {
		return fmt.Errorf("open directory %q: %w", targetDir, err)
	} else if !tdir_.IsDir() {
		return fmt.Errorf("%q is not a directory", targetDir)
	}

	hdm := hdmap.NewCellMapper(plugins, nil, nil)
	cellinfo, err := hdm.Generate(ctx)
	if err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}
	wdm := hdmap.NewWorldMapper()
	err = wdm.Write(ctx, hdm.MapExtents, cellinfo, &hdmap.NormalHeightImageSelector{}, filepath.Join(targetDir, "normalheightmap.bmp"))
	if err != nil {
		return fmt.Errorf("write world map: %w", err)
	}

	return nil
}

func main() {
	openmwcfg := "/home/ern/tes3/config/openmw.cfg"
	if err := sync(openmwcfg); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(33)
	}
}
