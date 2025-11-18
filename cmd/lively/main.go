package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
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
	env, err := cfg.Load(path)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}

	var rootPath string
	for _, plugin := range env.Plugins {
		if strings.EqualFold(filepath.Base(plugin), plugin_name) {
			rootPath = filepath.Dir(filepath.Dir(plugin))
		}
	}

	return drawMaps(ctx, rootPath, env)
}

func getRampFile(rootPath string) string {
	rampPath := filepath.Join(rootPath, "ramp.bmp")
	if f, err := os.Stat(rampPath); err != nil || f.IsDir() {
		fmt.Printf("Using built-in ramp file.\n")
		return ""
	}
	fmt.Printf("Using %q as the ramp file.\n", rampPath)
	return rampPath
}

func drawMaps(ctx context.Context, rootPath string, env *cfg.Environment) error {
	rampPath := getRampFile(rootPath)

	core00DataPath := filepath.Join(rootPath, "00 Core", "scripts", "LivelyMap", "data")
	core00TexturePath := filepath.Join(rootPath, "00 Core", "textures")
	detailTexturePath := filepath.Join(rootPath, "01 Detail Map", "textures")
	potatoTexturePath := filepath.Join(rootPath, "01 Potato Map", "textures")

	for _, texturePath := range []string{core00TexturePath, detailTexturePath, core00DataPath} {
		if tdir, err := os.Stat(texturePath); err != nil {
			return fmt.Errorf("open directory %q: %w", texturePath, err)
		} else if !tdir.IsDir() {
			return fmt.Errorf("%q is not a directory", texturePath)
		}
	}

	fmt.Printf("Parsing %d plugins...\n", len(env.Plugins))
	parsedLands := hdmap.NewLandParser(env)
	if err := parsedLands.ParsePlugins(); err != nil {
		return fmt.Errorf("parse plugins: %w", err)
	}
	fmt.Printf("Found %d land textures.\n", len(parsedLands.LandTextures))

	fmt.Printf("Done parsing %d cells.\n", len(parsedLands.Lands))

	// Render individual normal cells
	fmt.Printf("Rendering %d normalheightmap cells...\n", len(parsedLands.Lands))
	normalCells := hdmap.NewCellMapper(parsedLands, &hdmap.NormalHeightRenderer{})
	if err := normalCells.Generate(ctx); err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}
	// Render individual classic color cells
	fmt.Printf("Rendering %d classic color cells...\n", len(parsedLands.Lands))
	renderer, err := hdmap.NewClassicRenderer(rampPath)
	if err != nil {
		return fmt.Errorf("new classic renderer")
	}
	classicColorCells := hdmap.NewCellMapper(parsedLands, renderer)
	if err := classicColorCells.Generate(ctx); err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}
	// Render individual vertex color "detail" cells
	fmt.Printf("Rendering %d detailed cells...\n", len(parsedLands.Lands))
	texturedRenderer, err := hdmap.NewDetailRenderer(rampPath, parsedLands.LandTextures)
	if err != nil {
		return fmt.Errorf("new detailed renderer: %w", err)
	}
	texturedCells := hdmap.NewCellMapper(parsedLands, texturedRenderer)
	if err := texturedCells.Generate(ctx); err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}

	fmt.Printf("Setting up world map joiners...\n")

	// Set up jobs to join the sub-images together.
	mapInfos := []hdmap.SubmapNode{}
	maps := []*mapRenderJob{}
	for _, extents := range hdmap.Partition(parsedLands.MapExtents) {
		mapInfos = append(mapInfos, extents)

		maps = append(maps, &mapRenderJob{
			Directory: core00TexturePath,
			Name:      fmt.Sprintf("world_%d.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     classicColorCells,
			ScaleDown: 1,
		})
		maps = append(maps, &mapRenderJob{
			Directory: core00TexturePath,
			Name:      fmt.Sprintf("world_%d_nh.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     normalCells,
			ScaleDown: 4,
		})

		maps = append(maps, &mapRenderJob{
			Directory: potatoTexturePath,
			Name:      fmt.Sprintf("world_%d.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     classicColorCells,
			ScaleDown: 8,
		})
		maps = append(maps, &mapRenderJob{
			Directory: potatoTexturePath,
			Name:      fmt.Sprintf("world_%d_nh.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     normalCells,
			ScaleDown: 8,
		})

		maps = append(maps, &mapRenderJob{
			Directory: detailTexturePath,
			Name:      fmt.Sprintf("world_%d.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     texturedCells,
			ScaleDown: 1,
		})
	}

	// vanity map
	maps = append(maps, &mapRenderJob{
		Directory: rootPath,
		Name:      "vanity.png",
		Extents:   parsedLands.MapExtents,
		Cells:     texturedCells,
		ScaleDown: 1,
	})

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(6)
	for _, m := range maps {
		g.Go(func() error { return m.Draw(gctx) })
	}

	// Save map image info so the Lua mod knows what to do with them:
	g.Go(func() error {
		return printMapInfo(
			filepath.Join(core00DataPath, "maps.json"),
			mapInfos,
		)
	})

	return g.Wait()
}

func printMapInfo(path string, maps []hdmap.SubmapNode) error {
	container := struct {
		Maps []hdmap.SubmapNode
	}{
		Maps: maps,
	}
	raw, err := json.Marshal(container)
	if err != nil {
		return fmt.Errorf("marshal map info json: %w", err)
	}
	return os.WriteFile(path, raw, 0666)
}

type mapRenderJob struct {
	Directory string
	Name      string
	Extents   hdmap.MapCoords
	Cells     *hdmap.CellMapper
	ScaleDown int
}

func (m *mapRenderJob) Draw(ctx context.Context) error {
	fullPath := path.Join(m.Directory, m.Name)
	fmt.Printf("Combining cells for %q...\n", fullPath)
	classicWorldMapper := hdmap.NewWorldMapper()
	err := classicWorldMapper.Write(ctx,
		m.Extents,
		slices.Values(m.Cells.Cells),
		path.Join(m.Directory, m.Name),
		m.ScaleDown,
	)
	if err != nil {
		return fmt.Errorf("write world map %s %q: %w", m.Extents, m.Name, err)
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
