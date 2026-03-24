//go:build windows

package tunnel

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

// DefaultLogWriter is the default writer for loggers. Tests can override this
// to capture log output.
var DefaultLogWriter io.Writer

// LogPretty controls whether to use human-readable console output (true) or JSON (false).
var LogPretty bool

func init() {
	DefaultLogWriter = os.Stderr
}

// getWriter returns the appropriate writer based on LogPretty setting.
func getWriter(w io.Writer) io.Writer {
	if LogPretty {
		return zerolog.ConsoleWriter{Out: w, TimeFormat: simpleTimeFormat}
	}
	return w
}

// newPkgLogger creates the package-level logger using the current DefaultLogWriter and LogPretty.
func newPkgLogger() zerolog.Logger {
	return zerolog.New(getWriter(DefaultLogWriter)).With().Timestamp().Str("pkg", "tunnel").Logger()
}

// Set logging to use the file or syslog, one of the them must be "" else an error ensues
func makeLogger(pkg, file, facility string) zerolog.Logger {
	bootstrap := zerolog.New(getWriter(DefaultLogWriter)).With().Timestamp().Str("pkg", pkg).Logger()
	if file != "" {
		if facility != "" {
			bootstrap.Fatal().Msg("Can't log to syslog and logfile simultaneously")
		}
		bootstrap.Info().Str("file", file).Msg("Switching logging")
		f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			bootstrap.Fatal().Str("file", file).Err(err).Msg("Can't create log file")
		}
		logger := zerolog.New(f).With().Timestamp().Str("pkg", pkg).Logger()
		logger.Info().Msg("Started logging here")
		return logger
	} else if facility != "" {
		bootstrap.Warn().Str("facility", facility).Msg("Syslog is not supported on windows")
	} else {
		bootstrap.Info().Msg("WStunnel starting")
	}
	return bootstrap
}
