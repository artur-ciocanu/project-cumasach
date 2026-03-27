package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Activation struct {
	activePath  string
	backupPath  string
	hasBackup   bool
}

type PreparedSkill struct {
	ExtractedRoot string
	SkillName     string
}

func Activate(extractedRoot, targetDir, skillName string) (*Activation, error) {
	if err := ensureRuntimeVisibleTarget(targetDir); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("create target directory: %w", err)
	}

	activePath := filepath.Join(targetDir, skillName)
	tempPath, err := os.MkdirTemp(targetDir, "."+skillName+".activate-")
	if err != nil {
		return nil, fmt.Errorf("create activation staging directory: %w", err)
	}
	_ = os.RemoveAll(tempPath)

	if err := os.Rename(extractedRoot, tempPath); err != nil {
		return nil, fmt.Errorf("stage extracted skill: %w", err)
	}

	backupPath := ""
	hasBackup := false
	if _, err := os.Stat(activePath); err == nil {
		backupPath, err = os.MkdirTemp(targetDir, "."+skillName+".backup-")
		if err != nil {
			_ = os.RemoveAll(tempPath)
			return nil, fmt.Errorf("create activation backup directory: %w", err)
		}
		_ = os.RemoveAll(backupPath)
		if err := os.Rename(activePath, backupPath); err != nil {
			_ = os.RemoveAll(tempPath)
			return nil, fmt.Errorf("backup existing active skill: %w", err)
		}
		hasBackup = true
	}

	if err := os.Rename(tempPath, activePath); err != nil {
		if hasBackup {
			_ = os.Rename(backupPath, activePath)
		}
		_ = os.RemoveAll(tempPath)
		return nil, fmt.Errorf("activate skill: %w", err)
	}

	return &Activation{
		activePath: activePath,
		backupPath: backupPath,
		hasBackup:  hasBackup,
	}, nil
}

func ActivateAll(targetDir string, prepared []PreparedSkill) ([]*Activation, error) {
	if err := ensureRuntimeVisibleTarget(targetDir); err != nil {
		return nil, err
	}

	activations := make([]*Activation, 0, len(prepared))
	for _, skill := range prepared {
		activation, err := Activate(skill.ExtractedRoot, targetDir, skill.SkillName)
		if err != nil {
			for i := len(activations) - 1; i >= 0; i-- {
				_ = activations[i].Rollback()
			}
			return nil, err
		}
		activations = append(activations, activation)
	}
	return activations, nil
}

func RollbackAll(activations []*Activation) error {
	for i := len(activations) - 1; i >= 0; i-- {
		if err := activations[i].Rollback(); err != nil {
			return err
		}
	}
	return nil
}

func CommitAll(activations []*Activation) error {
	for _, activation := range activations {
		if err := activation.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (a *Activation) Rollback() error {
	if a == nil {
		return nil
	}
	if err := os.RemoveAll(a.activePath); err != nil {
		return fmt.Errorf("remove activated skill during rollback: %w", err)
	}
	if a.hasBackup {
		if err := os.Rename(a.backupPath, a.activePath); err != nil {
			return fmt.Errorf("restore previous active skill during rollback: %w", err)
		}
	}
	return nil
}

func (a *Activation) Commit() error {
	if a == nil || !a.hasBackup {
		return nil
	}
	if err := os.RemoveAll(a.backupPath); err != nil {
		return fmt.Errorf("remove activation backup: %w", err)
	}
	return nil
}

func ensureRuntimeVisibleTarget(targetDir string) error {
	info, err := os.Stat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat target directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("target path %q is not a directory", targetDir)
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return fmt.Errorf("read target directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !entry.IsDir() {
			return fmt.Errorf("runtime-visible entry %q is not a directory", name)
		}
	}

	return nil
}
