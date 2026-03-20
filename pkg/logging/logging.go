package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Setup initializes zerolog with the given level string and returns the logger.
// Supported levels: debug, info, warn, error. Defaults to info.
func Setup(level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	return zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).With().Timestamp().Logger()
}
