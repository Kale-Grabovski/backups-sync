package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backupsync/internal/sync"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync 7z backup files to backblaze",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cfgFile)
		if err != nil {
			return fmt.Errorf("config load failed: %w", err)
		}

		lgCfg := zap.NewProductionConfig()
		lgCfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")

		lg, err := lgCfg.Build()
		if err != nil {
			return fmt.Errorf("logger init failed: %w", err)
		}
		defer lg.Sync()

		lg.Info("sync started")

		blazeSync, err := sync.NewBackBlaze(cfg.BackBlaze, lg)
		if err != nil {
			lg.Error("cannot init backblaze manager", zap.Error(err))
			return err
		}

		if err := blazeSync.Run(); err != nil {
			lg.Error("initial backup cycle failed", zap.Error(err))
		}

		if cfg.BackBlaze.Interval <= 0 {
			lg.Info("sync finished (single run)")
			return nil
		}

		ticker := time.NewTicker(cfg.BackBlaze.Interval)
		defer ticker.Stop()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		lg.Info("entering daemon mode", zap.Duration("interval", cfg.BackBlaze.Interval))

		for {
			select {
			case <-ticker.C:
				lg.Info("starting scheduled backup cycle")
				if err := blazeSync.Run(); err != nil {
					lg.Error("backup cycle failed", zap.Error(err))
				}
			case sig := <-sigCh:
				lg.Info("received stop signal, shutting down gracefully", zap.String("signal", sig.String()))
				return nil
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
