// Package restore brings a backup back: list archives in Backblaze, download
// and decrypt one, then (separately and only on request) feed its pg_dump to
// a postgres container. It never overwrites the live system on its own:
// archives are unpacked into an empty directory, you copy files by hand.
package restore

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"backupsync/internal/config"

	"github.com/kothar/go-backblaze"
	"go.uber.org/zap"
)

type Archive struct {
	Name     string
	Size     int64
	Uploaded time.Time
	SHA1     string
}

type Restorer struct {
	lg     *zap.Logger
	bucket *backblaze.Bucket
	cfg    config.Config
}

// New connects to an EXISTING bucket. Unlike sync it never creates one: a typo
// in the bucket name must be an error, not a fresh empty bucket.
func New(lg *zap.Logger, cfg config.Config) (*Restorer, error) {
	client, err := backblaze.NewB2(backblaze.Credentials{
		AccountID:      cfg.BackBlaze.ID,
		ApplicationKey: cfg.BackBlaze.Key,
	})
	if err != nil {
		return nil, fmt.Errorf("backblaze auth: %w", err)
	}
	bucket, err := client.Bucket(cfg.BackBlaze.Bucket)
	if err != nil {
		return nil, fmt.Errorf("backblaze bucket: %w", err)
	}
	if bucket == nil {
		return nil, fmt.Errorf("bucket %q not found", cfg.BackBlaze.Bucket)
	}
	return &Restorer{lg: lg, bucket: bucket, cfg: cfg}, nil
}

// List returns the archives of this config's prefix, newest first.
func (r *Restorer) List() ([]Archive, error) {
	var out []Archive
	start := ""
	for {
		resp, err := r.bucket.ListFileNames(start, 1000)
		if err != nil {
			return nil, fmt.Errorf("list bucket: %w", err)
		}
		for _, f := range resp.Files {
			if !matches(f.Name, r.cfg.Backups.Prefix) {
				continue
			}
			out = append(out, Archive{
				Name:     f.Name,
				Size:     f.ContentLength,
				Uploaded: time.UnixMilli(f.UploadTimestamp),
				SHA1:     f.ContentSha1,
			})
		}
		if resp.NextFileName == "" {
			break
		}
		start = resp.NextFileName
	}
	sortNewest(out)
	return out, nil
}

func matches(name, prefix string) bool {
	return strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".7z")
}

func sortNewest(a []Archive) {
	sort.Slice(a, func(i, j int) bool { return a[i].Uploaded.After(a[j].Uploaded) })
}

// Pick returns the named archive, or the newest one when name is empty.
func Pick(archives []Archive, name string) (Archive, error) {
	if len(archives) == 0 {
		return Archive{}, fmt.Errorf("no archives in the bucket")
	}
	if name == "" {
		return archives[0], nil
	}
	for _, a := range archives {
		if a.Name == name {
			return a, nil
		}
	}
	return Archive{}, fmt.Errorf("archive %q not found (use --list)", name)
}

// Download saves the archive into dir (0600) and checks size and sha1.
func (r *Restorer) Download(a Archive, dir string) (string, error) {
	_, body, err := r.bucket.DownloadFileByName(a.Name)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", a.Name, err)
	}
	defer body.Close()

	path := filepath.Join(dir, a.Name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	n, err := io.Copy(io.MultiWriter(f, h), body)
	if err != nil {
		os.Remove(path)
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	if err := verify(n, hex.EncodeToString(h.Sum(nil)), a); err != nil {
		os.Remove(path)
		return "", err
	}
	r.lg.Info("archive downloaded", zap.String("file", path), zap.Int64("bytes", n))
	return path, nil
}

// verify compares what was written with what Backblaze reports. B2 reports
// "none" or an "unverified:" marker for large files: then only the size counts.
func verify(n int64, gotSHA1 string, a Archive) error {
	if a.Size > 0 && n != a.Size {
		return fmt.Errorf("size mismatch for %s: got %d, want %d", a.Name, n, a.Size)
	}
	want := strings.TrimPrefix(a.SHA1, "unverified:")
	if len(want) == 40 && !strings.EqualFold(want, gotSHA1) {
		return fmt.Errorf("sha1 mismatch for %s", a.Name)
	}
	return nil
}

// Extract unpacks into dir (relative paths, nothing is written outside it).
// The password is on the 7z command line, as it is when the archive is made.
func (r *Restorer) Extract(archive, dir string) error {
	bin := r.cfg.Backups.Path7z
	if bin == "" {
		var err error
		if bin, err = exec.LookPath("7z"); err != nil {
			return fmt.Errorf("7z binary not found: %w", err)
		}
	}
	cmd := exec.Command(bin, "x", "-p"+r.cfg.Backups.Pwd, "-o"+dir, "-y", archive)
	var stderr, stdout bytes.Buffer
	cmd.Stderr, cmd.Stdout = &stderr, &stdout
	if err := cmd.Run(); err != nil {
		hide := func(s string) string { return strings.ReplaceAll(s, r.cfg.Backups.Pwd, "***HIDDEN***") }
		return fmt.Errorf("7z x failed: %v: %s %s", err, hide(stderr.String()), hide(stdout.String()))
	}
	return nil
}

// FindDumps returns *.dump files under dir, newest first.
func FindDumps(dir string) ([]string, error) {
	type d struct {
		path string
		mod  time.Time
	}
	var found []d
	err := filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(e.Name(), ".dump") {
			return nil
		}
		if info, err := e.Info(); err == nil {
			found = append(found, d{p, info.ModTime()})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(found, func(i, j int) bool { return found[i].mod.After(found[j].mod) })
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.path
	}
	return out, nil
}

// PgRestore feeds a custom-format dump to pg_restore inside the container.
// apply=false only runs `pg_restore --list`: proves the dump is readable and
// touches nothing. apply=true drops and recreates the objects of the dump in
// the target database (--clean --if-exists), so it needs an explicit --yes.
func (r *Restorer) PgRestore(dump, container, user, database string, apply bool) (string, error) {
	args := []string{"exec", "-i", container, "pg_restore"}
	if apply {
		args = append(args, "-U", user, "-d", database, "--clean", "--if-exists", "--no-owner", "--no-acl")
	} else {
		args = append(args, "--list")
	}

	docker := "docker"
	if r.cfg.DB.DockerPath != "" {
		docker = r.cfg.DB.DockerPath
	}
	var cmd *exec.Cmd
	if r.cfg.DB.Sudo {
		cmd = exec.Command("sudo", append([]string{docker}, args...)...)
	} else {
		cmd = exec.Command(docker, args...)
	}

	in, err := os.Open(dump)
	if err != nil {
		return "", err
	}
	defer in.Close()

	var stdout, stderr bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pg_restore failed: %v: %s", err, stderr.String())
	}
	return stdout.String() + stderr.String(), nil
}
