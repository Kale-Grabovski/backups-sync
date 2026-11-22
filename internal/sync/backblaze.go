package sync

import (
	"os"
	"path/filepath"
	"time"

	"backupsync/internal/config"

	"github.com/kothar/go-backblaze"
	"go.uber.org/zap"
)

type BackBlaze struct {
	client *backblaze.B2
	bucket *backblaze.Bucket
	logger *zap.Logger
	config config.Backblaze
	// prefixes of the files of THIS host (backups.prefix, db.prefix). The bucket
	// is shared, the cleanup deletes only files that start with one of them.
	prefixes []string
}

func NewBackBlaze(config config.Backblaze, prefixes []string, logger *zap.Logger) (*BackBlaze, error) {
	client, err := backblaze.NewB2(backblaze.Credentials{
		AccountID:      config.ID,
		ApplicationKey: config.Key,
	})
	if err != nil {
		return nil, err
	}

	bucket, err := client.Bucket(config.Bucket)
	if err != nil {
		bucket, err = client.CreateBucket(config.Bucket, "allPrivate")
		if err != nil {
			return nil, err
		}
	}

	return &BackBlaze{
		client:   client,
		bucket:   bucket,
		logger:   logger,
		config:   config,
		prefixes: prefixes,
	}, nil
}

func (m *BackBlaze) Run() error {
	m.logger.Info("starting backup cycle")

	if err := m.uploadNewBackups(); err != nil {
		m.logger.Error("failed to upload backups", zap.Error(err))
		return err
	}

	if err := m.cleanupOldBackups(); err != nil {
		m.logger.Error("failed to cleanup old backups", zap.Error(err))
		return err
	}

	m.logger.Info("backup cycle completed")
	return nil
}

func (m *BackBlaze) uploadNewBackups() error {
	existingFiles, err := m.getRemoteFiles()
	if err != nil {
		return err
	}

	files, err := filepath.Glob(m.config.Path)
	if err != nil {
		return err
	}

	uploaded := 0
	for _, filePath := range files {
		fileName := filepath.Base(filePath)

		if existingFiles[fileName] {
			continue
		}

		// an archive past the bucket retention stays on disk longer
		// (backups.retention_days), and uploading it again would only make the
		// cleanup delete it again: it went back and forth every day
		info, err := os.Stat(filePath)
		if err != nil {
			m.logger.Error("failed to stat file", zap.String("file", fileName), zap.Error(err))
			continue
		}
		if expired(fileName, info.ModTime(), m.config.RetentionDays, time.Now()) {
			m.logger.Debug("skip: older than the bucket retention", zap.String("file", fileName))
			continue
		}

		m.logger.Info("uploading new backup", zap.String("file", fileName))

		file, err := os.Open(filePath)
		if err != nil {
			m.logger.Error("failed to open file", zap.String("file", fileName), zap.Error(err))
			continue
		}

		start := time.Now()
		_, err = m.bucket.UploadFile(fileName, nil, file)
		file.Close()

		if err != nil {
			m.logger.Error("failed to upload file", zap.String("file", fileName), zap.Error(err))
		} else {
			uploaded++
			m.logger.Info("file uploaded", zap.String("file", fileName), zap.Duration("duration", time.Since(start)))
		}
	}

	if uploaded > 0 {
		m.logger.Info("upload summary", zap.Int("uploaded_count", uploaded))
	}

	return nil
}

func (m *BackBlaze) cleanupOldBackups() error {
	files, err := m.listAll()
	if err != nil {
		return err
	}

	remote := make([]remoteFile, 0, len(files))
	for _, f := range files {
		remote = append(remote, remoteFile{
			Name:     f.Name,
			ID:       f.ID,
			Uploaded: time.UnixMilli(f.UploadTimestamp),
		})
	}

	deletedCount := 0
	for _, file := range selectExpired(remote, m.prefixes, m.config.RetentionDays, time.Now()) {
		m.logger.Info("deleting old backup", zap.String("file", file.Name), zap.Time("uploaded", file.Uploaded))

		if _, err = m.bucket.DeleteFileVersion(file.Name, file.ID); err != nil {
			m.logger.Error("failed to delete file", zap.String("file", file.Name), zap.Error(err))
		} else {
			deletedCount++
		}
	}

	m.logger.Info("cleanup completed", zap.Int("deleted_count", deletedCount))
	return nil
}

func (m *BackBlaze) getRemoteFiles() (map[string]bool, error) {
	files, err := m.listAll()
	if err != nil {
		return nil, err
	}

	existing := make(map[string]bool, len(files))
	for _, file := range files {
		existing[file.Name] = true
	}
	return existing, nil
}

// listAll walks the whole bucket page by page. One call returns at most 1000
// names, and a longer list silently dropped the rest: uploads were repeated
// and the cleanup never saw the old files.
func (m *BackBlaze) listAll() ([]backblaze.FileStatus, error) {
	var out []backblaze.FileStatus
	start := ""
	for {
		resp, err := m.bucket.ListFileNames(start, 1000)
		if err != nil {
			return nil, err
		}
		out = append(out, resp.Files...)
		if resp.NextFileName == "" {
			return out, nil
		}
		start = resp.NextFileName
	}
}
