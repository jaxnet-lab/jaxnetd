package corelog

import (
	"fmt"
	"io"
	"os"
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
	DisableConsoleLog bool `yaml:"disable_console_log" toml:"disable_console_log"`
	// LogsAsJSON makes the log framework log JSON
	LogsAsJSON bool `yaml:"logs_as_json" toml:"logs_as_json"`
	// FileLoggingEnabled makes the framework log to a file
	// the fields below can be skipped if this value is false!
	FileLoggingEnabled bool `yaml:"file_logging_enabled" toml:"file_logging_enabled"`
	// Directory to log to to when filelogging is enabled
	Directory string `yaml:"directory" toml:"directory"`
	// Filename is the name of the logfile which will be placed inside the directory
	Filename string `yaml:"filename" toml:"filename"`
	// MaxSize the max size in MB of the logfile before it's rolled
	MaxSize int `yaml:"max_size" toml:"max_size"`
	// MaxBackups the max number of rolled files to keep
	MaxBackups int `yaml:"max_backups" toml:"max_backups"`
	// MaxAge the max age in days to keep a logfile
	MaxAge  int  `yaml:"max_age" toml:"max_age"`
	NoColor bool `yaml:"no_color" toml:"no_color"`
}

const (
	defaultCfgMaxSize    = 1500
	defaultCfgMaxBackups = 3
	defaultCfgMaxAge     = 28
)

func (Config) Default() Config {
	return Config{
		DisableConsoleLog:  false,
		LogsAsJSON:         false,
		FileLoggingEnabled: false,
		Directory:          "core",
		Filename:           "jaxnetd.log",
		MaxSize:            defaultCfgMaxSize,
		MaxBackups:         defaultCfgMaxBackups,
		MaxAge:             defaultCfgMaxAge,
	}
}

type Logger struct {
	*zerolog.Logger
}

func consoleWriter(unit string, w io.Writer, noColor bool) zerolog.ConsoleWriter {
	out := zerolog.ConsoleWriter{Out: w, NoColor: noColor}
	out.TimeFormat = time.RFC3339
	out.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("| %-6s| %s |", i, unit))
	}
	out.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("%-6s  ", i)
	}
	return out
}

func New(unit string, logLevel zerolog.Level, config Config) zerolog.Logger {
	var writers []io.Writer

	if !config.DisableConsoleLog && !config.LogsAsJSON {
		writers = append(writers, consoleWriter(unit, os.Stderr, config.NoColor))
	}
	if !config.DisableConsoleLog && config.LogsAsJSON {
		writers = append(writers, os.Stdout)
	}
	if config.FileLoggingEnabled {
		writers = append(writers, newRollingFile(unit, config))
	}

	mw := io.MultiWriter(writers...)

	logger := zerolog.New(mw).
		Level(logLevel).
		With().
		Str("app", "jaxnetd").
		Timestamp().
		Logger()

	logger.Trace().
		Bool("fileLogging", config.FileLoggingEnabled).
		Bool("jsonLogOutput", config.LogsAsJSON).
		Str("logDirectory", config.Directory).
		Str("fileName", config.Filename).
		Int("maxSizeMB", config.MaxSize).
		Int("maxBackups", config.MaxBackups).
		Int("maxAgeInDays", config.MaxAge).
		Msg("logging configured")

	return logger
}

// nolint: gomnd
func newRollingFile(unit string, config Config) io.Writer {
	if err := os.MkdirAll(config.Directory, 0o744); err != nil {
		log.Error().Err(err).Str("path", config.Directory).Msg("can't create log directory")
		return nil
	}

	fileWriter := &lumberjack.Logger{
		Filename:   config.Filename,
		MaxBackups: config.MaxBackups, // files
		MaxSize:    config.MaxSize,    // megabytes
		MaxAge:     config.MaxAge,     // days
	}
	if config.LogsAsJSON {
		return fileWriter
	}

	return consoleWriter(unit, fileWriter, true)
}
