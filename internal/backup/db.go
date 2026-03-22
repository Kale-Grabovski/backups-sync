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

type DBackuper struct {
	lg  *zap.Logger
	cfg config.DB
}

func NewDBackuper(logger *zap.Logger, cfg config.DB) *DBackuper {
	return &DBackuper{
		lg:  logger,
		cfg: cfg,
	}
}

// CreateDump creates pg backup from docker container and saves it to a specified directory.
func (db *DBackuper) CreateDump() (string, error) {
	if db.cfg.Database == "" || db.cfg.User == "" || db.cfg.ContainerName == "" {
		return "", fmt.Errorf("database config incomplete: database=%s, user=%s, container=%s",
			db.cfg.Database, db.cfg.User, db.cfg.ContainerName)
	}

	// Generate output path FIRST, before any temp nonsense
	outPath := db.generateOutputPath()
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build the pg_dump command - docker exec handles the piping correctly
	pgDumpCmd := fmt.Sprintf("pg_dump -U %s -d %s -F c", db.cfg.User, db.cfg.Database)

	// Execute docker exec and read output directly
	docker := "docker"
	if db.cfg.DockerPath != "" {
		docker = db.cfg.DockerPath
	}
	cmd := exec.Command(docker, "exec", db.cfg.ContainerName, "sh", "-c", pgDumpCmd)

	// Write directly to output file, no temp nonsense
	outFile, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(outPath) // Clean up failed dump
		return "", fmt.Errorf("docker exec pg_dump failed: %v, details: %s", err, stderr.String())
	}

	db.lg.Info("database dump created successfully", zap.String("path", outPath))
	return outPath, nil
}

func (db *DBackuper) generateOutputPath() string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	dumpName := fmt.Sprintf("%s%s.dump", db.cfg.Prefix, timestamp)
	return filepath.Join(db.cfg.Path, dumpName)
}
