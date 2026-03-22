package backup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
