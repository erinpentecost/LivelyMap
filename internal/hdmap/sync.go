package hdmap

import (
	"context"
	"encoding/json"
	"fmt"
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

type annotatedDirectory struct {
	path      string
	available bool
}

func newAnnotatedDirectory(path string) (*annotatedDirectory, error) {
	if tdir, err := os.Stat(path); err != nil {
		fmt.Printf("Can't find %q!", path)
		return &annotatedDirectory{
			path:      path,
			available: false,
		}, nil
	} else if !tdir.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", path)
	}
	return &annotatedDirectory{
		path:      path,
		available: true,
	}, nil
}

func DrawMaps(ctx context.Context, rootPath string, env *cfg.Environment, maxThreads int, vanity bool, rampPath string) error {
	core00DataPath, err := newAnnotatedDirectory(filepath.Join(rootPath, "00 Core", "scripts", "LivelyMap", "data"))
	if err != nil {
		return err
	}
	classicTexturePath, err := newAnnotatedDirectory(filepath.Join(rootPath, "01 Classic Map", "textures", "LivelyMap"))
	if err != nil {
		return err
	}
	detailTexturePath, err := newAnnotatedDirectory(filepath.Join(rootPath, "01 Detail Map", "textures", "LivelyMap"))
	if err != nil {
		return err
	}
	potatoTexturePath, err := newAnnotatedDirectory(filepath.Join(rootPath, "01 Potato Map", "textures", "LivelyMap"))
	if err != nil {
		return err
	}
	colorTexturePath, err := newAnnotatedDirectory(filepath.Join(rootPath, "01 Color Map", "textures", "LivelyMap"))
	if err != nil {
		return err
	}
	normalsTexturePath, err := newAnnotatedDirectory(filepath.Join(rootPath, "02 Normals", "textures", "LivelyMap"))
	if err != nil {
		return err
	}
	extremeNormalsTexturePath, err := newAnnotatedDirectory(filepath.Join(rootPath, "02 Extreme Normals", "textures", "LivelyMap"))
	if err != nil {
		return err
	}

	texturePaths := []*annotatedDirectory{
		classicTexturePath,
		detailTexturePath,
		potatoTexturePath,
		colorTexturePath,
	}

	fmt.Printf("Parsing %d plugins...\n", len(env.Plugins))
	parsedLands := NewLandParser(env)
	if err := parsedLands.ParsePlugins(); err != nil {
		return fmt.Errorf("parse plugins: %w", err)
	}

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
	classicCells := NewCellMapper(parsedLands, renderer)
	if err := classicCells.Generate(ctx); err != nil {
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
	for _, path := range texturePaths {
		if path.available {
			if err := renderSky(path.path, renderer, specRenderer); err != nil {
				return fmt.Errorf("render sky texture: %w", err)
			}
		}
	}

	// Render individual vertex color "detail" cells
	fmt.Printf("Rendering %d detailed cells...\n", len(parsedLands.Lands))
	texturedRenderer, err := NewDetailRenderer(rampPath)
	if err != nil {
		return fmt.Errorf("new detailed renderer: %w", err)
	}
	texturedCells := NewCellMapper(parsedLands, texturedRenderer)
	if err := texturedCells.Generate(ctx); err != nil {
		return fmt.Errorf("generate cell maps: %w", err)
	}

	colorMapMaker, err := NewColorMapGenerator(ctx, rampPath, parsedLands)
	if err != nil {
		return fmt.Errorf("new color map generator: %w", err)
	}

	fmt.Printf("Setting up world map joiners...\n")

	// Set up jobs to join the sub-images together.

	mapInfos := map[string]SubmapNode{}
	mapJobs := []*mapRenderJob{}
	allHeights := map[string]float32{}
	for _, extents := range Partition(parsedLands.MapExtents) {
		mapInfos[strconv.Itoa(int(extents.ID))] = extents
		if classicTexturePath.available {
			mapJobs = append(mapJobs, &mapRenderJob{
				Directory: classicTexturePath.path,
				Name:      fmt.Sprintf("world_%d.dds", extents.ID),
				Extents:   extents.Extents,
				ProcessedWorldFn: simpleMapper(ctx, classicCells, extents.Extents,
					[]PostProcessor{
						&postprocessors.SMAA{},
						&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
					}),
				Codec: dds.Lossless,
			})
			mapJobs = append(mapJobs, &mapRenderJob{
				Directory: classicTexturePath.path,
				Name:      fmt.Sprintf("world_%d_spec.dds", extents.ID),
				Extents:   extents.Extents,
				ProcessedWorldFn: simpleMapper(ctx, specularCells, extents.Extents,
					[]PostProcessor{
						&postprocessors.SMAA{},
						&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
					}),
				Codec: dds.DXT5,
			})
		}
		if normalsTexturePath.available {
			mapJobs = append(mapJobs, &mapRenderJob{
				Directory: normalsTexturePath.path,
				Name:      fmt.Sprintf("world_%d_nh.dds", extents.ID),
				Extents:   extents.Extents,
				ProcessedWorldFn: simpleMapper(ctx, normalCells, extents.Extents,
					[]PostProcessor{
						&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
						&postprocessors.MinimumEdgeTransparencyProcessor{
							Minimum: 129,
						},
					}),
				Codec: dds.DXT5,
			})
		}
		if extremeNormalsTexturePath.available {
			mapJobs = append(mapJobs, &mapRenderJob{
				Directory: extremeNormalsTexturePath.path,
				Name:      fmt.Sprintf("world_%d_nh.dds", extents.ID),
				Extents:   extents.Extents,
				ProcessedWorldFn: simpleMapper(ctx, normalCells, extents.Extents,
					[]PostProcessor{
						&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
						&postprocessors.LocalToneMapAlpha{
							WindowRadiusDenom: 10,
						},
						&postprocessors.MinimumEdgeTransparencyProcessor{
							Minimum: 129,
						},
					}),
				Codec: dds.DXT5,
			})
		}

		if potatoTexturePath.available {
			mapJobs = append(mapJobs, &mapRenderJob{
				Directory: potatoTexturePath.path,
				Name:      fmt.Sprintf("world_%d.dds", extents.ID),
				Extents:   extents.Extents,
				ProcessedWorldFn: simpleMapper(ctx, classicCells, extents.Extents,
					[]PostProcessor{
						&postprocessors.SMAA{},
						&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 8},
					}),
				Codec: dds.DXT1,
			})
			mapJobs = append(mapJobs, &mapRenderJob{
				Directory: potatoTexturePath.path,
				Name:      fmt.Sprintf("world_%d_spec.dds", extents.ID),
				Extents:   extents.Extents,
				ProcessedWorldFn: simpleMapper(ctx, specularCells, extents.Extents,
					[]PostProcessor{
						&postprocessors.SMAA{},
						&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 8},
					}),
				Codec: dds.DXT5,
			})
		}

		if detailTexturePath.available {
			mapJobs = append(mapJobs, &mapRenderJob{
				Directory: detailTexturePath.path,
				Name:      fmt.Sprintf("world_%d.dds", extents.ID),
				Extents:   extents.Extents,
				ProcessedWorldFn: simpleMapper(ctx, texturedCells, extents.Extents,
					[]PostProcessor{
						&postprocessors.SMAA{},
						&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
					}),
				Codec: dds.Lossless,
			})
			mapJobs = append(mapJobs, &mapRenderJob{
				Directory: detailTexturePath.path,
				Name:      fmt.Sprintf("world_%d_spec.dds", extents.ID),
				Extents:   extents.Extents,
				ProcessedWorldFn: simpleMapper(ctx, specularCells, extents.Extents,
					[]PostProcessor{
						&postprocessors.SMAA{},
						&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
					}),
				Codec: dds.DXT5,
			})
		}
		if colorTexturePath.available {
			cm := func() (*WorldMapper, error) {
				return colorMapMaker.ProcessColorMapper(ctx, extents.Extents, []PostProcessor{
					&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
				})
			}
			mapJobs = append(mapJobs, &mapRenderJob{
				Directory:        colorTexturePath.path,
				Name:             fmt.Sprintf("world_%d.dds", extents.ID),
				Extents:          extents.Extents,
				ProcessedWorldFn: cm,
				Codec:            dds.Lossless,
			})
			mapJobs = append(mapJobs, &mapRenderJob{
				Directory: colorTexturePath.path,
				Name:      fmt.Sprintf("world_%d_spec.dds", extents.ID),
				Extents:   extents.Extents,
				ProcessedWorldFn: simpleMapper(ctx, specularCells, extents.Extents,
					[]PostProcessor{
						&postprocessors.SMAA{},
						&postprocessors.PowerOfTwoProcessor{DownScaleFactor: 1},
					}),
				Codec: dds.DXT5,
			})
		}
	}

	// vanity map
	if vanity {
		cm := func() (*WorldMapper, error) {
			return colorMapMaker.ProcessColorMapper(ctx, parsedLands.MapExtents, []PostProcessor{})
		}
		mapJobs = append(mapJobs, &mapRenderJob{
			Directory:        rootPath,
			Name:             "vanity.png",
			Extents:          parsedLands.MapExtents,
			ProcessedWorldFn: cm,
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
		filepath.Join(core00DataPath.path, "maps.json"),
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
	Directory        string
	Name             string
	Extents          MapCoords
	ProcessedWorldFn func() (*WorldMapper, error)
	//Cells          *CellMapper
	Codec dds.Codec
	//PostProcessors []PostProcessor
}

func (m *mapRenderJob) Draw(ctx context.Context) error {
	fullPath := path.Join(m.Directory, m.Name)
	fmt.Printf("Combining cells for %q...\n", fullPath)
	mapper, err := m.ProcessedWorldFn()
	if err != nil {
		return fmt.Errorf("process world map %s %q: %w", m.Extents, m.Name, err)
	}

	err = mapper.WriteOut(path.Join(m.Directory, m.Name), m.Codec)
	if err != nil {
		return fmt.Errorf("write world map %s %q: %w", m.Extents, m.Name, err)
	}
	return nil
}

func simpleMapper(ctx context.Context, cells *CellMapper, extents MapCoords, postProcs []PostProcessor) func() (*WorldMapper, error) {
	return func() (*WorldMapper, error) {
		classicWorldMapper := NewWorldMapper()
		err := classicWorldMapper.Process(ctx,
			extents,
			slices.Values(cells.Cells),
			postProcs,
		)
		if err != nil {
			return nil, fmt.Errorf("process world mapper: %w", err)
		}
		return classicWorldMapper, nil
	}
}
