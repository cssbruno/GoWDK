package appgen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/safeasset"
)

func validateDirectories(outputDir, appDir string) error {
	info, err := os.Stat(outputDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("build output %q is not a directory", outputDir)
	}
	rel, err := filepath.Rel(outputDir, appDir)
	if err != nil {
		return err
	}
	if rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..") {
		return fmt.Errorf("generated app directory %q must be outside build output directory %q", appDir, outputDir)
	}
	return nil
}

func isSameOrWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func copyOutputFiles(sourceRoot, targetRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		targetPath := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			if unsafeEmbeddedDirectory(rel) {
				return filepath.SkipDir
			}
			return os.MkdirAll(targetPath, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if !safeasset.EmbeddableGeneratedOutputFile(rel) {
			return nil
		}
		if err := copyFile(sourcePath, targetPath); err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	return files, err
}

func unsafeEmbeddedDirectory(rel string) bool {
	return safeasset.UnsafeEmbeddedDirectory(rel)
}

func copyFile(sourcePath, targetPath string) error {
	payload, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	return writeFileIfChanged(targetPath, payload)
}

func removeStaleOutputFiles(targetRoot string, files []string) error {
	keep := map[string]bool{}
	for _, file := range files {
		keep[file] = true
	}
	return filepath.WalkDir(targetRoot, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(targetRoot, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if keep[rel] {
			return nil
		}
		return os.Remove(filePath)
	})
}

func writeFileIfChanged(filePath string, contents []byte) error {
	current, err := os.ReadFile(filePath)
	if err == nil && bytes.Equal(current, contents) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filePath, contents, 0o644)
}
