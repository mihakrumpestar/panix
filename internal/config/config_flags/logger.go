package config_flags

import (
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Open log file
	file, err := os.OpenFile(flags.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
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
