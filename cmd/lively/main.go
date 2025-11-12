package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erinpentecost/LivelyMap/internal/hdmap"
	"github.com/ernmw/omwpacker/cfg"
	"golang.org/x/image/bmp"
)

const plugin_name = "livelymap.omwaddon"

func sync(path string) error {

	plugins, _, err := cfg.OpenMWPlugins(path)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	plugins = plugins

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

	// TODO remove, this is to speed up testing
	//plugins = plugins[:2]

	cellinfo, err := hdmap.RecordsToCellInfo(hdmap.LoadLANDs(plugins))
	if err != nil {
		return fmt.Errorf("parse land records: %w", err)
	}

	for cell := range cellinfo {
		targetName := fmt.Sprintf("%d_%d_nh.bmp", cell.X, cell.Y)
		fmt.Printf("Processing %q\n", targetName)
		if cell.NormalHeightMap == nil {
			fmt.Printf("%q has no normal height map\n", targetName)
			continue
		}
		out, err := os.Create(filepath.Join(targetDir, targetName))
		if err != nil {
			return fmt.Errorf("create file %q: %w", targetName, err)
		}
		defer out.Close()

		return bmp.Encode(out, cell.NormalHeightMap)
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
