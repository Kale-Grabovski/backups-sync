package config

type Config struct {
	BackBlaze Backblaze `mapstructure:"backblaze"`
}

type Backblaze struct {
	ID            string `mapstructure:"id"`
	Key           string `mapstructure:"key"`
	Bucket        string `mapstructure:"bucket"`
	Path          string `mapstructure:"path"`
	RetentionDays int    `mapstructure:"retention_days"`
}
