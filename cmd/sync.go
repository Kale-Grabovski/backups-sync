package cmd

import (
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
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := loadConfig(cfgFile)
		if err != nil {
			panic(err)
		}

		lgCfg := zap.NewProductionConfig()
		lgCfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
		lg, _ := lgCfg.Build()
		defer lg.Sync()

		lg.Info("Sync started", zap.Any("config", cfg))

		blazeSync, err := sync.NewBackBlaze(cfg.BackBlaze, lg)
		if err != nil {
			lg.Error("cannot init backblaze manager", zap.Error(err))
			return
		}

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		if err := blazeSync.Run(); err != nil {
			lg.Error("backup cycle failed", zap.Error(err))
		}

		for range ticker.C {
			if err := blazeSync.Run(); err != nil {
				lg.Error("backup cycle failed", zap.Error(err))
			}
		}

		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c

		lg.Info("Sync finished")
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
