package config

// LogConfig is the logger config.
type LogConfig struct {
	Level   string `mapstructure:"level"`   // debug | info | warn | error
	Dir     string `mapstructure:"dir"`     // directory for log files
	Console bool   `mapstructure:"console"` // whether to also output to the console
	Format  string `mapstructure:"format"`  // text | json
	Buffer  int    `mapstructure:"buffer"`  // async write buffer size; drop when full

	// File rolling strategy below.
	MaxSize          int    `mapstructure:"max_size"`          // per-file size limit (MB); roll once full
	MaxBackups       int    `mapstructure:"max_backups"`       // number of historical files to keep; 0 means unlimited
	MaxAge           int    `mapstructure:"max_age"`           // days to keep historical files; 0 means unlimited
	Compress         string `mapstructure:"compress"`          // none | gzip | zstd
	RotationInterval string `mapstructure:"rotation_interval"` // time-based rotation interval, e.g. 24h; when empty or 0, roll by size only
}
