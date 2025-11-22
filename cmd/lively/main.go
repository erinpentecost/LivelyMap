package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ernmw/omwpacker/cfg"
)

const plugin_name = "livelymap.omwaddon"

func sync(ctx context.Context, path string) error {
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

	return drawMaps(ctx, rootPath, env)
}

func main() {
	// debug
	openmwcfg := "/home/ern/tes3/config/openmw.cfg"
	if len(os.Args) > 1 {
		openmwcfg = os.Args[1]
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer cancel()

	if err := sync(ctx, openmwcfg); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(33)
	}
}
