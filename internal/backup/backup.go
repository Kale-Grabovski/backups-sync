package backup

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"backupsync/internal/config"

	"go.uber.org/zap"
)

type Archiver struct {
	logger *zap.Logger
	cfg    config.Backups
}

func NewArchiver(logger *zap.Logger, cfg config.Backups) *Archiver {
	return &Archiver{
		logger: logger,
		cfg:    cfg,
	}
}

func (a *Archiver) CreateArchive() (string, error) {
	if len(a.cfg.Inputs) == 0 {
		return "", fmt.Errorf("empty inputs")
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	archiveName := fmt.Sprintf(a.cfg.Prefix+"%s.7z", timestamp)
	outPath := filepath.Join(a.cfg.Output, archiveName)

	a.logger.Info("starting gazen archive", zap.String("archive", outPath))

	args := []string{"a", "-p" + a.cfg.Pwd, "-mhe=on", "-mx=9", "-y", outPath}
	args = append(args, a.cfg.Inputs...)

	binPath, err := exec.LookPath("7z")
	if err != nil {
		if a.cfg.Path7z != "" {
			binPath = a.cfg.Path7z
		} else {
			return "", fmt.Errorf("7z isn't installed, fuck you: %w", err)
		}
	}

	a.logger.Info("found 7z", zap.String("path", binPath))
	cmd := exec.Command(binPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		safeErr := strings.ReplaceAll(stderr.String(), a.cfg.Pwd, "***HIDDEN***")
		return "", fmt.Errorf("7z error: %v, details: %s", err, safeErr)
	}

	a.logger.Info("archive created", zap.String("path", outPath))
	return outPath, nil
}
