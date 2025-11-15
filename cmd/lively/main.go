package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/erinpentecost/LivelyMap/internal/hdmap"
	"github.com/ernmw/omwpacker/cfg"
	"golang.org/x/sync/errgroup"
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

	return drawMaps(ctx, targetDir, plugins)
}

func drawMaps(ctx context.Context, targetDir string, plugins []string) error {
	fmt.Printf("Parsing %d plugins...\n", len(plugins))
	parsedLands := hdmap.NewLandParser(plugins)
	if err := parsedLands.ParsePlugins(); err != nil {
		return fmt.Errorf("parse plugins: %w", err)
	}
	fmt.Printf("Done parsing %d cells.\n", len(parsedLands.Lands))

	// Render individual cells
	fmt.Printf("Rendering %d normalheightmap cells...\n", len(parsedLands.Lands))
	normalCells := hdmap.NewCellMapper(parsedLands, &hdmap.NormalHeightRenderer{})
	if err := normalCells.Generate(ctx); err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}
	// Render individual cells
	fmt.Printf("Rendering %d classic color cells...\n", len(parsedLands.Lands))
	renderer, err := hdmap.NewClassicRenderer("")
	if err != nil {
		return fmt.Errorf("new classic renderer")
	}
	classicColorCells := hdmap.NewCellMapper(parsedLands, renderer)
	if err := classicColorCells.Generate(ctx); err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(4)

	for _, extents := range hdmap.FindSquares(parsedLands.MapExtents) {
		g.Go(func() error {
			fmt.Printf("Combining cells for extent %s...\n", extents)
			normalWorldMapper := hdmap.NewWorldMapper()
			err := normalWorldMapper.Write(
				ctx,
				extents,
				slices.Values(normalCells.Cells),
				filepath.Join(targetDir, fmt.Sprintf("world_%s_nh.dds", extents)))
			if err != nil {
				return fmt.Errorf("write world map: %w", err)
			}
			return nil
		})

		g.Go(func() error {
			fmt.Printf("Combining cells for extent %s...\n", extents)
			classicWorldMapper := hdmap.NewWorldMapper()
			err = classicWorldMapper.Write(ctx,
				extents,
				slices.Values(classicColorCells.Cells),
				filepath.Join(targetDir, fmt.Sprintf("world_%s.dds", extents)))
			if err != nil {
				return fmt.Errorf("write world map: %w", err)
			}
			return nil
		})
	}

	return g.Wait()
}

func main() {
	openmwcfg := "/home/ern/tes3/config/openmw.cfg"
	if err := sync(openmwcfg); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(33)
	}
}
