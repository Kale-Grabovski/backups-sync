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

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Creates PostgreSQL database dumps on a schedule",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := loadConfig(cfgFile)
		if err != nil {
			panic(err)
		}

		if cfg.DB.Database == "" || cfg.DB.User == "" {
			panic("database and user must be configured in db section")
		}

		lgCfg := zap.NewProductionConfig()
		lgCfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
		lg, _ := lgCfg.Build()
		defer lg.Sync()

		lg.Info("database backup worker started",
			zap.String("container", cfg.DB.ContainerName),
			zap.String("database", cfg.DB.Database),
			zap.Duration("interval", cfg.DB.Interval),
			zap.Int("retention_days", cfg.DB.RetentionDays))

		backuper := backup.NewDBackuper(lg, cfg.DB)

		doBackup := func() {
			outPath, err := backuper.CreateDump()
			if err != nil {
				lg.Error("error creating database dump", zap.Error(err))
				return
			}
			lg.Info("database dump done", zap.String("file", outPath))

			if err := backuper.CleanupOldBackups(); err != nil {
				lg.Error("error cleaning up old backups", zap.Error(err))
			}
		}

		if cfg.DB.Immediate {
			doBackup()
		}

		if cfg.DB.Interval <= 0 {
			lg.Info("database backup finished (single run)")
			return
		}

		ticker := time.NewTicker(cfg.DB.Interval)
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
				lg.Info("database backup worker stopped")
				return
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
}
