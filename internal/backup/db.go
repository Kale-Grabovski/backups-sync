package backup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"backupsync/internal/config"

	"go.uber.org/zap"
)

// DBackuper represents a superior German engineering tool for database backups
type DBackuper struct {
	lg  *zap.Logger
	cfg config.DB
}

// NewDBackuper initializes the backup machinery
func NewDBackuper(logger *zap.Logger, cfg config.DB) *DBackuper {
	return &DBackuper{
		lg:  logger,
		cfg: cfg,
	}
}

// CreateDump executes the backup process. Failure is not an option.
func (db *DBackuper) CreateDump() (string, error) {
	if db.cfg.Database == "" || db.cfg.User == "" || db.cfg.ContainerName == "" {
		return "", fmt.Errorf("database config incomplete: database=%s, user=%s, container=%s",
			db.cfg.Database, db.cfg.User, db.cfg.ContainerName)
	}

	outPath := db.generateOutputPath()
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	dockerCmd := "docker"
	if db.cfg.DockerPath != "" {
		dockerCmd = db.cfg.DockerPath
	}

	// pass arguments directly to pg_dump to avoid shell escaping nightmare
	execArgs := []string{
		"exec",
		"-i",
		db.cfg.ContainerName,
		"pg_dump",
		"-U", db.cfg.User,
		"-d", db.cfg.Database,
		"-F", "c",
	}

	var cmd *exec.Cmd
	if db.cfg.Sudo {
		sudoArgs := append([]string{dockerCmd}, execArgs...)
		cmd = exec.Command("sudo", sudoArgs...)
	} else {
		cmd = exec.Command(dockerCmd, execArgs...)
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	db.lg.Debug("executing backup command", zap.String("command", cmd.String()))

	if err := cmd.Run(); err != nil {
		outFile.Close()    // Close before removing
		os.Remove(outPath) // Burn the evidence of failure
		return "", fmt.Errorf("pg_dump execution failed: %v, stderr: %s", err, stderr.String())
	}

	db.lg.Info("database dump created successfully", zap.String("path", outPath))
	return outPath, nil
}

func (db *DBackuper) generateOutputPath() string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	dumpName := fmt.Sprintf("%s%s.dump", db.cfg.Prefix, timestamp)
	return filepath.Join(db.cfg.Path, dumpName)
}

// CleanupOldBackups removes backups older than RetentionDays
func (db *DBackuper) CleanupOldBackups() error {
	if db.cfg.RetentionDays > 0 {
		if err := db.cleanByAge(); err != nil {
			return err
		}
	}
	if db.cfg.KeepLast > 0 {
		return db.trimToKeepLast()
	}
	return nil
}

// DumpAndCleanup makes a dump and applies the retention. When the dump fails
// the old dumps are left alone: a broken pg_dump must not eat the last good one.
func (db *DBackuper) DumpAndCleanup() (string, error) {
	outPath, err := db.CreateDump()
	if err != nil {
		return "", err
	}
	if err := db.CleanupOldBackups(); err != nil {
		db.lg.Error("error cleaning up old backups", zap.Error(err))
	}
	return outPath, nil
}

// trimToKeepLast removes all dumps but the KeepLast newest ones.
func (db *DBackuper) trimToKeepLast() error {
	entries, err := os.ReadDir(db.cfg.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	type dump struct {
		name string
		mod  time.Time
	}
	var dumps []dump
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".dump") {
			continue
		}
		if db.cfg.Prefix != "" && !strings.HasPrefix(name, db.cfg.Prefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		dumps = append(dumps, dump{name, info.ModTime()})
	}

	// Newest first, the name breaks a tie (it carries the timestamp).
	sort.Slice(dumps, func(i, j int) bool {
		if !dumps[i].mod.Equal(dumps[j].mod) {
			return dumps[i].mod.After(dumps[j].mod)
		}
		return dumps[i].name > dumps[j].name
	})

	for _, d := range dumps[min(db.cfg.KeepLast, len(dumps)):] {
		if err := os.Remove(filepath.Join(db.cfg.Path, d.name)); err != nil {
			db.lg.Warn("failed to delete old dump", zap.String("file", d.name), zap.Error(err))
			continue
		}
		db.lg.Info("deleted dump over keep_last", zap.String("file", d.name))
	}
	return nil
}

func (db *DBackuper) cleanByAge() error {
	cutoffTime := time.Now().AddDate(0, 0, -db.cfg.RetentionDays)
	entries, err := os.ReadDir(db.cfg.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	deletedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if db.cfg.Prefix != "" && !strings.HasPrefix(name, db.cfg.Prefix) {
			continue
		}
		if !strings.HasSuffix(name, ".dump") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			path := filepath.Join(db.cfg.Path, name)
			if err := os.Remove(path); err != nil {
				db.lg.Warn("failed to delete old backup", zap.String("file", name), zap.Error(err))
			} else {
				deletedCount++
				db.lg.Info("deleted old backup", zap.String("file", name), zap.Time("modified", info.ModTime()))
			}
		}
	}

	if deletedCount > 0 {
		db.lg.Info("cleanup completed", zap.Int("deleted_count", deletedCount))
	}

	return nil
}
