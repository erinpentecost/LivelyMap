package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"flag"

	"github.com/erinpentecost/LivelyMap/internal/hdmap"
	"github.com/erinpentecost/LivelyMap/internal/savefile"
	"github.com/ernmw/omwpacker/cfg"
)

const plugin_name = "livelymap.omwaddon"

var openmwCfgPath = flag.String("cfg", "./openmw.cfg", "full path to your openmw.cfg file")
var threads = flag.Int("threads", 6, "number of threads to use. reduce this if you run out of memory.")
var mapTextures = flag.Bool("maps", true, "create map textures")
var saveFiles = flag.Bool("saves", true, "extract paths from save files")
var vanity = flag.Bool("vanity", false, "generate full vanity map")
var rampPath = flag.String("ramp", "classic", "full path to a ramp file, or one of: classic,gold,light,purple")

func init() {
	flag.Parse()
	fmt.Printf("cfg: %q\n", *openmwCfgPath)
	fmt.Printf("threads: %d\n", *threads)
	fmt.Printf("maps: %v\n", *mapTextures)
	fmt.Printf("saveFiles: %v\n", *saveFiles)
	fmt.Printf("vanity: %v\n", *vanity)
	fmt.Printf("rampPath: %q\n", *rampPath)
}

func sync(ctx context.Context) error {
	path := *openmwCfgPath
	env, err := cfg.Load(path)
	if err != nil {
		return fmt.Errorf("load openmw.cfg: %w", err)
	}

	var rootPath string
	for _, plugin := range env.Plugins {
		if strings.EqualFold(filepath.Base(plugin), plugin_name) {
			rootPath = filepath.Dir(filepath.Dir(plugin))
		}
	}

	if *mapTextures {
		if err := hdmap.DrawMaps(ctx, rootPath, env, *threads, *vanity, *rampPath); err != nil {
			return fmt.Errorf("draw maps: %w", err)
		}
	}

	if *saveFiles {
		if err := savefile.ExtractSaveData(rootPath, env); err != nil {
			return fmt.Errorf("extract save data: %w", err)
		}
	}

	return nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancel()

	if err := sync(ctx); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(33)
	}
}
