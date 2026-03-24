//go:build !windows

package tunnel

import (
	"io"
	"log/syslog"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// parseSyslogFacility converts a facility name string to its syslog.Priority constant.
func parseSyslogFacility(facility string) (syslog.Priority, bool) {
	switch strings.ToLower(strings.TrimSpace(facility)) {
	case "kern":
		return syslog.LOG_KERN, true
	case "user":
		return syslog.LOG_USER, true
	case "mail":
		return syslog.LOG_MAIL, true
	case "daemon":
		return syslog.LOG_DAEMON, true
	case "auth":
		return syslog.LOG_AUTH, true
	case "syslog":
		return syslog.LOG_SYSLOG, true
	case "lpr":
		return syslog.LOG_LPR, true
	case "news":
		return syslog.LOG_NEWS, true
	case "uucp":
		return syslog.LOG_UUCP, true
	case "cron":
		return syslog.LOG_CRON, true
	case "authpriv":
		return syslog.LOG_AUTHPRIV, true
	case "ftp":
		return syslog.LOG_FTP, true
	case "local0":
		return syslog.LOG_LOCAL0, true
	case "local1":
		return syslog.LOG_LOCAL1, true
	case "local2":
		return syslog.LOG_LOCAL2, true
	case "local3":
		return syslog.LOG_LOCAL3, true
	case "local4":
		return syslog.LOG_LOCAL4, true
	case "local5":
		return syslog.LOG_LOCAL5, true
	case "local6":
		return syslog.LOG_LOCAL6, true
	case "local7":
		return syslog.LOG_LOCAL7, true
	default:
		return 0, false
	}
}

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
		priority := syslog.LOG_DAEMON | syslog.LOG_INFO
		if parsed, ok := parseSyslogFacility(facility); ok {
			priority = parsed | syslog.LOG_INFO
		} else {
			bootstrap.Warn().Str("facility", facility).Msg("Unknown syslog facility, using daemon")
		}
		bootstrap.Info().Str("facility", facility).Msg("Switching logging to syslog")
		w, err := syslog.New(priority, pkg)
		if err != nil {
			bootstrap.Fatal().Err(err).Msg("Can't connect to syslog")
		}
		logger := zerolog.New(zerolog.SyslogLevelWriter(w)).With().Str("pkg", pkg).Logger()
		logger.Info().Msg("Started logging here")
		return logger
	}
	bootstrap.Info().Msg("WStunnel starting")
	return bootstrap
}
