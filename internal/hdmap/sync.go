package hdmap

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/erinpentecost/LivelyMap/internal/dds"
	"github.com/erinpentecost/LivelyMap/internal/hdmap/postprocessors"
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

func DrawMaps(ctx context.Context, rootPath string, env *cfg.Environment, maxThreads int, vanity bool) error {
	rampPath := getRampFile(rootPath)

	core00DataPath := filepath.Join(rootPath, "00 Core", "scripts", "LivelyMap", "data")
	core00TexturePath := filepath.Join(rootPath, "00 Core", "textures", "LivelyMap")
	detailTexturePath := filepath.Join(rootPath, "01 Detail Map", "textures", "LivelyMap")
	potatoTexturePath := filepath.Join(rootPath, "01 Potato Map", "textures", "LivelyMap")
	normalsTexturePath := filepath.Join(rootPath, "02 Normals", "textures", "LivelyMap")
	extremeNormalsTexturePath := filepath.Join(rootPath, "02 Extreme Normals", "textures", "LivelyMap")

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

	fmt.Printf("Rendering %d specular cells...\n", len(parsedLands.Lands))
	specRenderer, err := NewSpecularRenderer()
	if err != nil {
		return fmt.Errorf("new specular renderer")
	}
	specularCells := NewCellMapper(parsedLands, specRenderer)
	if err := specularCells.Generate(ctx); err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}

	// Special "sky" cell
	if err := renderSky(core00TexturePath, renderer, specRenderer); err != nil {
		return fmt.Errorf("render sky texture: %w", err)
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

	mapInfos := map[string]SubmapNode{}
	mapJobs := []*mapRenderJob{}
	allHeights := map[string]float32{}
	for _, extents := range Partition(parsedLands.MapExtents) {
		mapInfos[strconv.Itoa(int(extents.ID))] = extents
		mapJobs = append(mapJobs, &mapRenderJob{
			Directory: core00TexturePath,
			Name:      fmt.Sprintf("world_%d.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     classicColorCells,
			PostProcessors: []PostProcessor{
				&postprocessors.SMAA{},
				&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
			},
			Codec: dds.Lossless,
		})
		mapJobs = append(mapJobs, &mapRenderJob{
			Directory: normalsTexturePath,
			Name:      fmt.Sprintf("world_%d_nh.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     normalCells,
			PostProcessors: []PostProcessor{
				&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
				&postprocessors.MinimumEdgeTransparencyProcessor{
					Minimum: 255,
				},
			},
			Codec: dds.DXT5,
		})
		mapJobs = append(mapJobs, &mapRenderJob{
			Directory: extremeNormalsTexturePath,
			Name:      fmt.Sprintf("world_%d_nh.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     normalCells,
			PostProcessors: []PostProcessor{
				&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
				&postprocessors.LocalToneMapAlpha{
					WindowRadiusDenom: 10,
				},
				&postprocessors.MinimumEdgeTransparencyProcessor{
					Minimum: 255,
				},
			},
			Codec: dds.DXT5,
		})
		mapJobs = append(mapJobs, &mapRenderJob{
			Directory: core00TexturePath,
			Name:      fmt.Sprintf("world_%d_spec.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     specularCells,
			PostProcessors: []PostProcessor{
				&postprocessors.SMAA{},
				&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
			},
			Codec: dds.DXT5,
		})

		mapJobs = append(mapJobs, &mapRenderJob{
			Directory: potatoTexturePath,
			Name:      fmt.Sprintf("world_%d.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     classicColorCells,
			PostProcessors: []PostProcessor{
				&postprocessors.SMAA{},
				&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 8},
			},
			Codec: dds.DXT1,
		})
		mapJobs = append(mapJobs, &mapRenderJob{
			Directory: potatoTexturePath,
			Name:      fmt.Sprintf("world_%d_spec.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     specularCells,
			PostProcessors: []PostProcessor{
				&postprocessors.SMAA{},
				&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 8},
			},
			Codec: dds.DXT5,
		})

		mapJobs = append(mapJobs, &mapRenderJob{
			Directory: detailTexturePath,
			Name:      fmt.Sprintf("world_%d.dds", extents.ID),
			Extents:   extents.Extents,
			Cells:     texturedCells,
			PostProcessors: []PostProcessor{
				&postprocessors.SMAA{},
				&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
			},
			Codec: dds.Lossless,
		})
	}

	// vanity map
	if vanity {
		mapJobs = append(mapJobs, &mapRenderJob{
			Directory: rootPath,
			Name:      "vanity.png",
			Extents:   parsedLands.MapExtents,
			Cells:     texturedCells,
			PostProcessors: []PostProcessor{
				&postprocessors.SMAA{},
			},
		})
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxThreads)
	for _, m := range mapJobs {
		g.Go(func() error { return m.Draw(gctx) })
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("generate textures: %w", err)
	}

	// Save map image info so the Lua mod knows what to do with them:
	return printMapInfo(
		filepath.Join(core00DataPath, "maps.json"),
		parsedLands,
		mapInfos,
		allHeights,
	)
}

func renderSky(textureFolder string, colorRenderer CellRenderer, specularRenderer *SpecularRenderer) error {
	{
		skyImg := colorRenderer.Render(NewFallbackLandRecord())
		fullPath := path.Join(textureFolder, "sky.dds")
		out, err := os.Create(fullPath)
		if err != nil {
			return fmt.Errorf("create %q: %w", fullPath, err)
		}
		if err := dds.Encode(out, skyImg, dds.DXT1); err != nil {
			return fmt.Errorf("encode sky texture: %w", err)
		}
	}
	{
		skyImgSpec := specularRenderer.Render(NewFallbackLandRecord())
		fullPath := path.Join(textureFolder, "sky_spec.dds")
		out, err := os.Create(fullPath)
		if err != nil {
			return fmt.Errorf("create %q: %w", fullPath, err)
		}
		if err := dds.Encode(out, skyImgSpec, dds.DXT5); err != nil {
			return fmt.Errorf("encode sky texture: %w", err)
		}
	}
	return nil
}

func printMapInfo(path string, parsedLands *LandParser, maps map[string]SubmapNode, allHeights map[string]float32) error {
	container := struct {
		Maps      map[string]SubmapNode
		MaxHeight float64
		Heights   map[string]float32
	}{
		Maps:      maps,
		MaxHeight: parsedLands.MaxHeight,
		Heights:   allHeights,
	}
	raw, err := json.Marshal(container)
	if err != nil {
		return fmt.Errorf("marshal map info json: %w", err)
	}
	return os.WriteFile(path, raw, 0666)
}

type mapRenderJob struct {
	Directory      string
	Name           string
	Extents        MapCoords
	Cells          *CellMapper
	Codec          dds.Codec
	PostProcessors []PostProcessor
	PostFunction   func(img *image.RGBA) error
}

func (m *mapRenderJob) Draw(ctx context.Context) error {
	fullPath := path.Join(m.Directory, m.Name)
	fmt.Printf("Combining cells for %q...\n", fullPath)
	classicWorldMapper := NewWorldMapper()
	err := classicWorldMapper.Write(ctx,
		m.Extents,
		slices.Values(m.Cells.Cells),
		path.Join(m.Directory, m.Name),
		m.PostProcessors,
		m.Codec,
	)
	if err != nil {
		return fmt.Errorf("write world map %s %q: %w", m.Extents, m.Name, err)
	}
	if m.PostFunction != nil {
		err := m.PostFunction(classicWorldMapper.outImage)
		if err != nil {
			return fmt.Errorf("PostFunction: %w", err)
		}
	}
	return nil
}
