package backup

import (
	"bytes"
	"crypto/sha256"
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

const (
	cronDumpPath  = "/tmp/.crontab_backup"
	duplicatesAge = 48 * time.Hour
)

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
		return "", fmt.Errorf("empty inputs, nothing to archive")
	}

	a.dumpCrontab()

	fullState := a.calcInputsState()
	shortHash := fullState[:8]

	if !a.isBackupNeeded(shortHash) {
		return "", nil
	}

	outPath := a.generateOutputPath(shortHash)
	if err := a.execute7z(outPath); err != nil {
		return "", err
	}

	a.lg.Info("stateless archive created successfully", zap.String("path", outPath))
	return outPath, nil
}

func (a *Archiver) isBackupNeeded(currentHash string) bool {
	pattern := filepath.Join(a.cfg.Output, a.cfg.Prefix+"*.7z")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		a.lg.Info("no previous archives found, backup is REQUIRED")
		return true
	}

	sort.Strings(matches)
	lastArchive := matches[len(matches)-1]

	base := strings.TrimSuffix(filepath.Base(lastArchive), ".7z")
	parts := strings.Split(base, "_")
	lastHash := ""
	if len(parts) > 0 {
		lastHash = parts[len(parts)-1]
	}

	var age = 999 * time.Hour
	if info, err := os.Stat(lastArchive); err == nil {
		age = time.Since(info.ModTime())
	}

	if lastHash == currentHash && age < duplicatesAge {
		a.lg.Info("stateless check: files haven't changed and backup is fresh. STAND DOWN!",
			zap.String("hash", currentHash),
			zap.Duration("age", age))
		return false
	}

	a.lg.Info("changes detected or backup is old, backup is REQUIRED",
		zap.String("new_hash", currentHash),
		zap.Duration("last_age", age))
	return true
}

func (a *Archiver) execute7z(outPath string) error {
	a.lg.Info("arming 7z...", zap.String("archive", outPath))

	binPath, err := exec.LookPath("7z")
	if err != nil {
		if a.cfg.Path7z != "" {
			binPath = a.cfg.Path7z
		} else {
			return fmt.Errorf("7z binary not found: %w", err)
		}
	}

	args := []string{"a", "-spf2", "-p" + a.cfg.Pwd, "-mhe=on", "-mx=9", "-y", outPath}
	args = append(args, a.cfg.Inputs...)

	cmd := exec.Command(binPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		safeErr := strings.ReplaceAll(stderr.String(), a.cfg.Pwd, "***HIDDEN***")
		return fmt.Errorf("7z error: %v, details: %s", err, safeErr)
	}

	return nil
}

func (a *Archiver) generateOutputPath(shortHash string) string {
	timestamp := time.Now().Format("2006-01-02_15-04")
	archiveName := fmt.Sprintf("%s%s_%s.7z", a.cfg.Prefix, timestamp, shortHash)
	return filepath.Join(a.cfg.Output, archiveName)
}

func (a *Archiver) calcInputsState() string {
	h := sha256.New()
	for _, path := range a.cfg.Inputs {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			if p == cronDumpPath {
				content, readErr := os.ReadFile(p)
				if readErr == nil {
					fmt.Fprintf(h, "%s|content:%s\n", p, string(content))
					return nil
				}
				a.lg.Warn("could not read crontab dump for hashing, falling back to mtime", zap.Error(readErr))
			}

			fmt.Fprintf(h, "%s|%d|%d\n", p, info.Size(), info.ModTime().UnixNano())
			return nil
		})
	}
	return fmt.Sprintf("%x", h.Sum(nil))
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
