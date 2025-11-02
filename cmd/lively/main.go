package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "extract" {
		fmt.Println("")
		fmt.Println("Usage: wnamtool extract -i <input plugin, openmw.cfg, or morrowind.ini path> -b <bmp output dir> [--esm]")
		fmt.Println("Example: wnamtool extract -i Morrowind.esm -b ./out")
		os.Exit(1)
	}

	// Parse flags after "extract"
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	inPath := fs.String("i", "", "input plugin, openmw.cfg, or morrowind.ini path")
	bmpDir := fs.String("b", "", "bmp output directory")
	esmFlag := fs.Bool("esm", false, "only read master files")
	fs.Parse(os.Args[2:])

	if *inPath == "" || *bmpDir == "" {
		fmt.Println("Both -i and -b are required.")
		os.Exit(1)
	}

	i := *inPath
	b := *bmpDir

	// Determine content files mapping
	contentFiles := make(map[string]string)
	ext := strings.ToLower(filepath.Ext(i))
	if ext == ".esp" || ext == ".esm" || ext == ".omwaddon" {
		base := strings.ToLower(filepath.Base(i))
		contentFiles[base] = i
	} else if ext == ".cfg" {
		res, err := openMWPlugins(i, *esmFlag)
		if err != nil {
			fmt.Println("Error reading cfg:", err)
			os.Exit(1)
		}
		if res == nil {
			fmt.Println("No content files found in cfg.")
			os.Exit(1)
		}
		contentFiles = res
	} else if ext == ".ini" {
		res, err := MWPlugins(i, *esmFlag)
		if err != nil {
			fmt.Println("Error reading ini:", err)
			os.Exit(1)
		}
		if res == nil {
			fmt.Println("No plugins found in ini.")
			os.Exit(1)
		}
		contentFiles = res
	} else {
		fmt.Println("Unsupported input file type:", ext)
		os.Exit(1)
	}

	// Ensure bmpDir exists
	if _, err := os.Stat(b); os.IsNotExist(err) {
		if err := os.MkdirAll(b, 0755); err != nil {
			fmt.Println("Unable to create output directory:", err)
			os.Exit(1)
		}
	}

	msg, err := pluginsToBMP(contentFiles, b)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println(msg)
}
