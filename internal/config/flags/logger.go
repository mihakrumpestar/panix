package flags

import (
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	DefaultLogFilePermissions os.FileMode = 0644
	DefaultDirPermissions     os.FileMode = 0755
)

func InitLogging(flags Logging) error {
	// Determine if logging should be enabled
	enabled := flags.Log || flags.Debug

	if !enabled {
		// Logging is disabled - set global level to disable all logging
		zerolog.SetGlobalLevel(zerolog.Disabled)

		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(flags.LogFile)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, DefaultDirPermissions); err != nil {
			return errors.Wrap(err, "failed to create log directory")
		}
	}

	// Open log file (we don't close it, and that is ok)
	file, err := os.OpenFile(flags.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, DefaultLogFilePermissions)
	if err != nil {
		return errors.Wrap(err, "failed to open log file")
	}

	// Set global log level
	if flags.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Configure global logger with console writer for pretty printing to file
	// NoColor is set to true to avoid ANSI color codes in the log file
	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        file,
		NoColor:    true,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	log.Info().Str("log_file", flags.LogFile).Msg("Logger initialized")

	return nil
}
