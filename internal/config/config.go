package config

import "time"

type Config struct {
	BackBlaze Backblaze `mapstructure:"backblaze"`
	Backups   Backups   `mapstructure:"backups"`
}

type Backups struct {
	Inputs   []string      `mapstructure:"inputs"`
	Path7z   string        `mapstructure:"path_7z"`
	Pwd      string        `mapstructure:"pwd"`
	Output   string        `mapstructure:"output"`
	Prefix   string        `mapstructure:"prefix"`
	Interval time.Duration `mapstructure:"interval"`
}

type Backblaze struct {
	ID            string `mapstructure:"id"`
	Key           string `mapstructure:"key"`
	Bucket        string `mapstructure:"bucket"`
	Path          string `mapstructure:"path"`
	RetentionDays int    `mapstructure:"retention_days"`
}
