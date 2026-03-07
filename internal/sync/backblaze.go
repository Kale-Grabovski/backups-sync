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
}

func NewBackBlaze(config config.Backblaze, logger *zap.Logger) (*BackBlaze, error) {
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
		client: client,
		bucket: bucket,
		logger: logger,
		config: config,
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
	resp, err := m.bucket.ListFileNames("", 1000)
	if err != nil {
		return err
	}

	cutoffTime := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	deletedCount := 0

	for _, file := range resp.Files {
		uploadTime := time.Unix(file.UploadTimestamp/1000, 0)
		if uploadTime.Before(cutoffTime) {
			m.logger.Info("deleting old backup", zap.String("file", file.Name), zap.Time("uploaded", uploadTime))

			if _, err = m.bucket.DeleteFileVersion(file.Name, file.ID); err != nil {
				m.logger.Error("failed to delete file", zap.String("file", file.Name), zap.Error(err))
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		m.logger.Info("cleanup completed", zap.Int("deleted_count", deletedCount))
	}

	return nil
}

func (m *BackBlaze) getRemoteFiles() (map[string]bool, error) {
	resp, err := m.bucket.ListFileNames("", 1000)
	if err != nil {
		return nil, err
	}

	existing := make(map[string]bool)
	for _, file := range resp.Files {
		existing[file.Name] = true
	}
	return existing, nil
}
