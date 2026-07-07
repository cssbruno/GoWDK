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

type plannedFile struct {
	path     string
	contents []byte
}

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
	files, planned, err := collectOutputFiles(sourceRoot, targetRoot)
	if err != nil {
		return nil, err
	}
	for _, file := range planned {
		if err := writeFileIfChanged(file.path, file.contents); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func collectOutputFiles(sourceRoot, targetRoot string) ([]string, []plannedFile, error) {
	var files []string
	var planned []plannedFile
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
			return nil
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
		payload, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		planned = append(planned, plannedFile{path: targetPath, contents: payload})
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	return files, planned, err
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
	temp, err := os.CreateTemp(filepath.Dir(filePath), "."+filepath.Base(filePath)+".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := temp.Write(contents); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, filePath); err != nil {
		return err
	}
	cleanup = false
	return nil
}
