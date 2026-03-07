package cmd

import (
	"fmt"
	"strings"

	"backupsync/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use: "bsync",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "app.yaml", "config yaml file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func loadConfig(path string) (cfg config.Config, err error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	err = viper.ReadInConfig()
	if err != nil {
		return cfg, fmt.Errorf("cannot read config: %w", err)
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err = viper.Unmarshal(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("cannot unmarshal config: %w", err)
	}
	return
}
