package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/erinpentecost/LivelyMap/internal/heightmap"
)

const plugin_name = "livelymap.omwscripts"

func sync(path string) error {

	plugins, err := heightmap.OpenMWPlugins(path, false)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	// find path to dump map to
	for _, plugin := range plugins {
		print(plugin.Name + "\n")
		if plugin.Name == "livelymap.omwscripts" {
			targetPath := filepath.Join(filepath.Dir(plugin.Path), "scripts", "LivelyMap", "dump")

			// build map
			mm := heightmap.NewMapMaker()
			if _, err := mm.PluginsToBMP(plugins, targetPath); err != nil {
				return fmt.Errorf("dump map image to %q: %w", path, err)
			}
			return nil
		}
	}
	return fmt.Errorf("can't find %q", plugin_name)
}

func main() {
	openmwcfg := "/home/ern/tes3/config/openmw.cfg"
	if err := sync(openmwcfg); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(33)
	}
}
