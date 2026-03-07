package backup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"backupsync/internal/config"

	"go.uber.org/zap"
)

const cronDumpPath = "/tmp/.crontab_backup"

type Archiver struct {
	lg  *zap.Logger
	cfg config.Backups
}

func NewArchiver(logger *zap.Logger, cfg config.Backups) *Archiver {
	return &Archiver{
		lg:  logger,
		cfg: cfg,
	}
}

func (a *Archiver) CreateArchive() (string, error) {
	if len(a.cfg.Inputs) == 0 {
		return "", fmt.Errorf("empty inputs")
	}

	timestamp := time.Now().Format("2006-01-02_15-04")
	archiveName := fmt.Sprintf(a.cfg.Prefix+"%s.7z", timestamp)
	outPath := filepath.Join(a.cfg.Output, archiveName)

	a.lg.Info("starting gazen archive", zap.String("archive", outPath))

	args := []string{"a", "-spf2", "-p" + a.cfg.Pwd, "-mhe=on", "-mx=9", "-y", outPath}
	args = append(args, a.cfg.Inputs...)

	a.dumpCrontab()

	binPath, err := exec.LookPath("7z")
	if err != nil {
		if a.cfg.Path7z != "" {
			binPath = a.cfg.Path7z
		} else {
			return "", fmt.Errorf("7z isn't installed, fuck you: %w", err)
		}
	}

	a.lg.Info("found 7z", zap.String("path", binPath))
	cmd := exec.Command(binPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		safeErr := strings.ReplaceAll(stderr.String(), a.cfg.Pwd, "***HIDDEN***")
		return "", fmt.Errorf("7z error: %v, details: %s", err, safeErr)
	}

	a.lg.Info("archive created", zap.String("path", outPath))
	return outPath, nil
}

func (a *Archiver) dumpCrontab() {
	a.lg.Info("executing tactical gazen crontab dump")

	cmd := exec.Command("crontab", "-l")
	out, err := cmd.Output()
	if err != nil {
		a.lg.Warn("no crontab found or command failed, generating dummy file for 7z", zap.Error(err))
		out = []byte("# No crontab found for this user\n")
	}

	if err := os.WriteFile(cronDumpPath, out, 0644); err != nil {
		a.lg.Error("failed to write crontab dump", zap.Error(err))
		return
	}

	a.lg.Info("crontab successfully dumped (or stubbed) to", zap.String("path", cronDumpPath))
}
