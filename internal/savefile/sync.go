package savefile

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ernmw/omwpacker/cfg"
)

func newestFileInFolder(path string, ext string) (fs.FileInfo, error) {
	saveFiles, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read dirs in %q: %w", path, err)
	}
	var latestSave fs.FileInfo
	for _, saveFile := range saveFiles {
		saveInfo, err := saveFile.Info()
		if err != nil {
			return nil, fmt.Errorf("read file info in %q: %w", path, err)
		}
		if saveInfo.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(saveFile.Name()), ext) {
			continue
		}
		if latestSave == nil || saveInfo.ModTime().After(latestSave.ModTime()) {
			latestSave = saveInfo
		}
	}
	return latestSave, nil
}

func ExtractSaveData(
	rootPath string,
	env *cfg.Environment) error {
	for _, userDir := range env.User {
		saveDir := filepath.Join(userDir, "saves")
		entries, err := os.ReadDir(saveDir)
		if err != nil {
			return fmt.Errorf("read dirs in %q: %w", saveDir, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			characterDir := filepath.Join(saveDir, entry.Name())
			newestSave, err := newestFileInFolder(characterDir, ".omwsave")
			if err != nil {
				return fmt.Errorf("newest file in folder: %w", err)
			}
			if newestSave == nil {
				continue
			}
			// have we already dumped this character?
			dumpPath := filepath.Join(rootPath,
				"00 Core",
				"scripts",
				"LivelyMap",
				"data",
				"paths",
				fmt.Sprintf("%s.json", entry.Name()))
			var parsedExistingData *SaveData
			{
				existingData, _ := os.ReadFile(dumpPath) // drop error
				if len(existingData) > 0 {
					parsedExistingData, err = Unmarshal(existingData)
					if err != nil {
						return fmt.Errorf("bad path data in %q: %w", dumpPath, err)
					}
				}
			}

			// this next call will edit the save file
			newestSaveFileName := filepath.Join(characterDir, newestSave.Name())
			data, err := ExtractData(newestSaveFileName)
			if err != nil {
				// no data to extract.
				fmt.Printf("extract save data in %q: %v\n", newestSaveFileName, err)
				continue
			}
			newData, err := Merge(parsedExistingData, data)
			if err != nil {
				return fmt.Errorf("merge %q and %q: %w", dumpPath, newestSaveFileName, err)
			}
			marshalledNewData, err := json.Marshal(newData)
			if err != nil {
				return fmt.Errorf("marshal merged data for %q: %w", dumpPath, err)
			}
			if err := os.WriteFile(dumpPath, marshalledNewData, 0666); err != nil {
				return fmt.Errorf("persist path data for %q: %w", dumpPath, err)
			}
			fmt.Printf("Extracted path data from %q to %q.", newestSaveFileName, dumpPath)
		}
	}
	return nil
}
