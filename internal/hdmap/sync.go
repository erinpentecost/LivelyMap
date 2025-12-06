package hdmap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/erinpentecost/LivelyMap/internal/dds"
	"github.com/ernmw/omwpacker/cfg"
	"golang.org/x/sync/errgroup"
)

func getRampFile(rootPath string) string {
	rampPath := filepath.Join(rootPath, "ramp.bmp")
	if f, err := os.Stat(rampPath); err != nil || f.IsDir() {
		fmt.Printf("Using built-in ramp file.\n")
		return ""
	}
	fmt.Printf("Using %q as the ramp file.\n", rampPath)
	return rampPath
}

func DrawMaps(ctx context.Context, rootPath string, env *cfg.Environment) error {
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
	parsedLands := NewLandParser(env)
	if err := parsedLands.ParsePlugins(); err != nil {
		return fmt.Errorf("parse plugins: %w", err)
	}
	fmt.Printf("Found %d land textures.\n", len(parsedLands.LandTextures))

	fmt.Printf("Done parsing %d cells.\n", len(parsedLands.Lands))

	// Render individual normal cells
	fmt.Printf("Rendering %d normalheightmap cells...\n", len(parsedLands.Lands))
	normalCells := NewCellMapper(parsedLands, &NormalHeightRenderer{})
	if err := normalCells.Generate(ctx); err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}
	// Render individual classic color cells
	fmt.Printf("Rendering %d classic color cells...\n", len(parsedLands.Lands))
	renderer, err := NewClassicRenderer(rampPath)
	if err != nil {
		return fmt.Errorf("new classic renderer")
	}
	classicColorCells := NewCellMapper(parsedLands, renderer)
	if err := classicColorCells.Generate(ctx); err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}
	// Special "sky" cell
	{
		skyImg := renderer.Render(NewFallbackLandRecord())
		fullPath := path.Join(core00TexturePath, "sky.dds")
		out, err := os.Create(fullPath)
		if err != nil {
			return fmt.Errorf("create %q: %w", fullPath, err)
		}
		if err := dds.Encode(out, skyImg); err != nil {
			return fmt.Errorf("encode sky texture: %w", err)
		}
	}

	// Render individual vertex color "detail" cells
	fmt.Printf("Rendering %d detailed cells...\n", len(parsedLands.Lands))
	texturedRenderer, err := NewDetailRenderer(rampPath, parsedLands.LandTextures)
	if err != nil {
		return fmt.Errorf("new detailed renderer: %w", err)
	}
	texturedCells := NewCellMapper(parsedLands, texturedRenderer)
	if err := texturedCells.Generate(ctx); err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}

	fmt.Printf("Setting up world map joiners...\n")

	// Set up jobs to join the sub-images together.
	mapInfos := []SubmapNode{}
	maps := []*mapRenderJob{}
	for _, extents := range Partition(parsedLands.MapExtents) {
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

func printMapInfo(path string, maps []SubmapNode) error {
	container := struct {
		Maps []SubmapNode
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
	Extents   MapCoords
	Cells     *CellMapper
	ScaleDown int
}

func (m *mapRenderJob) Draw(ctx context.Context) error {
	fullPath := path.Join(m.Directory, m.Name)
	fmt.Printf("Combining cells for %q...\n", fullPath)
	classicWorldMapper := NewWorldMapper()
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
