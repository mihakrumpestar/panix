package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func InitLogging(logging flags.Logging, output flags.OutputMode) error {
	headless := output != flags.OutputModeTui
	fileLogging := logging.Log || logging.Debug

	if !fileLogging && !headless {
		zerolog.SetGlobalLevel(zerolog.Disabled)

		return nil
	}

	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.DurationFieldUnit = time.Second

	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if logging.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	w, logFilePath, err := buildWriter(logging.LogFile, output, fileLogging, headless)
	if err != nil {
		return err
	}

	log.Logger = zerolog.New(w).With().Timestamp().Logger()

	initMsg := log.Info() //nolint:zerologlint
	if logFilePath != "" {
		initMsg = initMsg.Str("log_file", logFilePath)
	}

	initMsg.Str("output", string(output)).
		Bool("debug", logging.Debug).
		Msg("logger initialized")

	return nil
}

func buildWriter(logFile string, output flags.OutputMode, fileLogging, headless bool) (io.Writer, string, error) {
	var (
		writers     []io.Writer
		logFilePath string
	)

	if fileLogging {
		file, path, err := openLogFile(logFile)
		if err != nil {
			return nil, "", err
		}

		logFilePath = path

		writers = append(writers, file)
	}

	if headless {
		writers = append(writers, stdoutWriter(output))
	}

	if len(writers) == 1 {
		return writers[0], logFilePath, nil
	}

	return zerolog.MultiLevelWriter(writers...), logFilePath, nil
}

func openLogFile(logFile string) (io.Writer, string, error) {
	path := getLogFilePath(logFile)

	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		err := os.MkdirAll(dir, filepermissions.DefaultDirPermissions)
		if err != nil {
			return nil, "", errors.Wrap(err, "failed to create log directory")
		}
	}

	//nolint:gosec // This is ok
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filepermissions.DefaultFilePermissions)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to open log file")
	}

	return file, path, nil
}

func stdoutWriter(output flags.OutputMode) io.Writer {
	if output == flags.OutputModeJSON {
		return os.Stdout
	}

	return zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    !flags.IsTerminal(),
	}
}

func getLogFilePath(logFile string) string {
	now := time.Now().Unix()

	ext := filepath.Ext(logFile)
	if ext == "" {
		return fmt.Sprintf("%s.%d", logFile, now)
	}

	return fmt.Sprintf("%s.%d%s", strings.TrimSuffix(logFile, ext), now, ext)
}

func ResultEvent(l zerolog.Logger, msg string, err error, extra func(event *zerolog.Event)) {
	var evt *zerolog.Event
	if err != nil {
		evt = l.Error()
	} else {
		evt = l.Info()
	}

	if extra != nil {
		extra(evt)
	}

	if err != nil {
		evt = evt.Str("status", "failed").Str("error", err.Error())
	} else {
		evt = evt.Str("status", "success")
	}

	evt.Msg(msg)
}
