package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backupsync/internal/backup"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Archives a list of folders to 7z on a schedule",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := loadConfig(cfgFile)
		if err != nil {
			panic(err)
		}

		lgCfg := zap.NewProductionConfig()
		lgCfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
		lg, _ := lgCfg.Build()
		defer lg.Sync()

		lg.Info("backup worker started", zap.Duration("interval", cfg.Backups.Interval))

		archiver := backup.NewArchiver(lg, cfg.Backups)

		doBackup := func() {
			outPath, err := archiver.CreateArchive()
			if err != nil {
				lg.Error("error creating backup", zap.Error(err))
				return
			}
			lg.Info("backup done", zap.String("file", outPath))

			if cfg.Backups.RetentionDays > 0 {
				if err := archiver.CleanupOldArchives(); err != nil {
					lg.Error("error cleaning up old archives", zap.Error(err))
				}
			}
		}

		doBackup()

		if cfg.Backups.Interval <= 0 {
			lg.Info("backup finished (single run)")
			return
		}

		ticker := time.NewTicker(cfg.Backups.Interval)
		defer ticker.Stop()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		go func() {
			<-sigCh
			lg.Info("finishing...")
			cancel()
		}()

		for {
			select {
			case <-ticker.C:
				doBackup()
			case <-ctx.Done():
				lg.Info("backup worker stopped")
				return
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(backupCmd)
}
