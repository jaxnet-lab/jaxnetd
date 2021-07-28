package corelog

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Disabled zerolog.Logger

	DefaultLevel   = zerolog.InfoLevel
	DefaultLogFile = "jaxnetd.log"
)

func init() {
	Disabled = zerolog.Nop()
}

// Config for logging
type Config struct {
	// Disable console logging
	DisableConsoleLog bool `yaml:"disable_console_log"`
	// LogsAsJson makes the log framework log JSON
	LogsAsJson bool `yaml:"logs_as_json"`
	// FileLoggingEnabled makes the framework log to a file
	// the fields below can be skipped if this value is false!
	FileLoggingEnabled bool `yaml:"file_logging_enabled"`
	// Directory to log to to when filelogging is enabled
	Directory string `yaml:"directory"`
	// Filename is the name of the logfile which will be placed inside the directory
	Filename string `yaml:"filename"`
	// MaxSize the max size in MB of the logfile before it's rolled
	MaxSize int `yaml:"max_size"`
	// MaxBackups the max number of rolled files to keep
	MaxBackups int `yaml:"max_backups"`
	// MaxAge the max age in days to keep a logfile
	MaxAge int `yaml:"max_age"`
}

func (Config) Default() Config {
	return Config{
		DisableConsoleLog:  false,
		LogsAsJson:         false,
		FileLoggingEnabled: false,
		Directory:          "core",
		Filename:           "jaxnetd.log",
		MaxSize:            150,
		MaxBackups:         3,
		MaxAge:             28,
	}
}

type Logger struct {
	*zerolog.Logger
}

func New(unit string, logLevel zerolog.Level, config Config) zerolog.Logger {
	var writers []io.Writer
	if !config.DisableConsoleLog && !config.LogsAsJson {
		out := zerolog.ConsoleWriter{Out: os.Stderr, NoColor: false}
		out.TimeFormat = time.RFC3339
		out.FormatLevel = func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("| %-6s| %s |", i, unit))
		}
		out.FormatMessage = func(i interface{}) string {
			return fmt.Sprintf("%-6s  ", i)
		}
		writers = append(writers, out)
	}
	if !config.DisableConsoleLog && config.LogsAsJson {
		writers = append(writers, os.Stdout)
	}
	if config.FileLoggingEnabled {
		writers = append(writers, newRollingFile(config))
	}

	mw := io.MultiWriter(writers...)
	zerolog.SetGlobalLevel(DefaultLevel)

	logger := zerolog.New(mw).
		Level(logLevel).
		With().
		Str("app", "jaxnetd").
		Timestamp().
		Logger()

	logger.Trace().
		Bool("fileLogging", config.FileLoggingEnabled).
		Bool("jsonLogOutput", config.LogsAsJson).
		Str("logDirectory", config.Directory).
		Str("fileName", config.Filename).
		Int("maxSizeMB", config.MaxSize).
		Int("maxBackups", config.MaxBackups).
		Int("maxAgeInDays", config.MaxAge).
		Msg("logging configured")

	return logger
}

func newRollingFile(config Config) io.Writer {
	if err := os.MkdirAll(config.Directory, 0744); err != nil {
		log.Error().Err(err).Str("path", config.Directory).Msg("can't create log directory")
		return nil
	}

	return &lumberjack.Logger{
		Filename:   path.Join(config.Directory, config.Filename),
		MaxBackups: config.MaxBackups, // files
		MaxSize:    config.MaxSize,    // megabytes
		MaxAge:     config.MaxAge,     // days
	}
}
