package log

// Config is the confuration struct for the log package.
//
// Can be deserialized from YAML.
type Config struct {
	// Level is the log level you want to set your service to.
	Level Level `yaml:"level"`
}

// InitFromConfig initializes the log package using the given Config and JSON
// logger.
func InitFromConfig(cfg Config) {
	if cfg.Level == "" {
		cfg.Level = InfoLevel
	}
	level := cfg.Level
	InitLoggerJSON(level)
}
