// Package publish stages and commits generated artifact generations.
package publish

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Transaction owns same-filesystem staging paths for one or more generated
// directories or files. Commit replaces all targets and rolls every completed
// replacement back if a later replacement fails.
type Transaction struct {
	entries   []entry
	committed bool
}

type entry struct {
	target string
	stage  string
	backup string
	isDir  bool
	moved  bool
	hadOld bool
}

// StageDirectory creates an empty sibling staging directory for target.
func (tx *Transaction) StageDirectory(target string) (string, error) {
	absolute, err := cleanTarget(target)
	if err != nil {
		return "", err
	}
	if err := Recover(absolute); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(absolute), "."+filepath.Base(absolute)+".gowdk-stage-*")
	if err != nil {
		return "", err
	}
	tx.entries = append(tx.entries, entry{target: absolute, stage: stage, isDir: true})
	return stage, nil
}

// StageFile creates a sibling staging file path for target. The empty file is
// ready for a generator or compiler to overwrite.
func (tx *Transaction) StageFile(target string) (string, error) {
	absolute, err := cleanTarget(target)
	if err != nil {
		return "", err
	}
	if err := Recover(absolute); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(filepath.Dir(absolute), "."+filepath.Base(absolute)+".gowdk-stage-*")
	if err != nil {
		return "", err
	}
	stage := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(stage)
		return "", err
	}
	tx.entries = append(tx.entries, entry{target: absolute, stage: stage})
	return stage, nil
}

// Commit replaces every target. Existing targets are retained as backups until
// every replacement succeeds, then removed. A failed replacement restores the
// earlier generation before returning.
func (tx *Transaction) Commit() error {
	if tx.committed {
		return fmt.Errorf("publication transaction already committed")
	}
	if len(tx.entries) == 0 {
		return fmt.Errorf("publication transaction has no staged entries")
	}
	for index := range tx.entries {
		item := &tx.entries[index]
		if err := validateStage(*item); err != nil {
			_ = tx.rollback(index - 1)
			return err
		}
		backup, err := unusedBackupPath(item.target)
		if err != nil {
			_ = tx.rollback(index - 1)
			return err
		}
		item.backup = backup
		if _, err := os.Lstat(item.target); err == nil {
			if err := os.Rename(item.target, item.backup); err != nil {
				_ = tx.rollback(index - 1)
				return fmt.Errorf("backup committed generation %q: %w", item.target, err)
			}
			item.hadOld = true
		} else if !os.IsNotExist(err) {
			_ = tx.rollback(index - 1)
			return err
		}
		if err := os.Rename(item.stage, item.target); err != nil {
			if item.hadOld {
				_ = os.Rename(item.backup, item.target)
			}
			_ = tx.rollback(index - 1)
			return fmt.Errorf("publish staged generation %q: %w", item.target, err)
		}
		item.moved = true
	}
	tx.committed = true
	for index := range tx.entries {
		item := &tx.entries[index]
		if item.hadOld {
			// A backup-cleanup failure does not turn an already committed
			// generation into a failed build. Recover removes it next time.
			_ = os.RemoveAll(item.backup)
		}
	}
	return nil
}

// Recover repairs publication debris left by an interrupted process. If a
// target vanished after its old generation was backed up, the newest backup is
// restored. Otherwise obsolete backups and stages are removed.
func Recover(target string) error {
	absolute, err := cleanTarget(target)
	if err != nil {
		return err
	}
	dir, base := filepath.Dir(absolute), filepath.Base(absolute)
	backups, err := filepath.Glob(filepath.Join(dir, "."+base+".gowdk-backup-*"))
	if err != nil {
		return err
	}
	stages, err := filepath.Glob(filepath.Join(dir, "."+base+".gowdk-stage-*"))
	if err != nil {
		return err
	}
	if _, targetErr := os.Lstat(absolute); os.IsNotExist(targetErr) && len(backups) > 0 {
		sort.SliceStable(backups, func(i, j int) bool {
			left, leftErr := os.Lstat(backups[i])
			right, rightErr := os.Lstat(backups[j])
			if leftErr != nil || rightErr != nil || left.ModTime().Equal(right.ModTime()) {
				return backups[i] < backups[j]
			}
			return left.ModTime().Before(right.ModTime())
		})
		newest := backups[len(backups)-1]
		if err := os.Rename(newest, absolute); err != nil {
			return fmt.Errorf("recover publication target %q: %w", absolute, err)
		}
		backups = backups[:len(backups)-1]
	} else if targetErr != nil && !os.IsNotExist(targetErr) {
		return targetErr
	}
	for _, path := range append(backups, stages...) {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove stale publication path %q: %w", path, err)
		}
	}
	return nil
}

// Abort removes uncommitted staging paths. It is safe to call with defer.
func (tx *Transaction) Abort() {
	if tx == nil || tx.committed {
		return
	}
	for _, item := range tx.entries {
		if item.moved {
			continue
		}
		_ = os.RemoveAll(item.stage)
	}
}

func (tx *Transaction) rollback(last int) error {
	var failures []error
	for index := last; index >= 0; index-- {
		item := &tx.entries[index]
		if !item.moved {
			continue
		}
		if err := os.RemoveAll(item.target); err != nil {
			failures = append(failures, err)
			continue
		}
		if item.hadOld {
			if err := os.Rename(item.backup, item.target); err != nil {
				failures = append(failures, err)
			}
		}
		item.moved = false
	}
	return errors.Join(failures...)
}

func cleanTarget(target string) (string, error) {
	if strings.TrimSpace(target) == "" {
		return "", fmt.Errorf("publication target is required")
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if absolute == filepath.Dir(absolute) {
		return "", fmt.Errorf("refusing to publish filesystem root %q", absolute)
	}
	return filepath.Clean(absolute), nil
}

func validateStage(item entry) error {
	info, err := os.Lstat(item.stage)
	if err != nil {
		return fmt.Errorf("staged generation %q is unavailable: %w", item.stage, err)
	}
	if item.isDir != info.IsDir() {
		return fmt.Errorf("staged generation %q has unexpected file type", item.stage)
	}
	if filepath.Dir(item.stage) != filepath.Dir(item.target) {
		return fmt.Errorf("staged generation %q is not on the target filesystem", item.stage)
	}
	return nil
}

func unusedBackupPath(target string) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".gowdk-backup-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}
