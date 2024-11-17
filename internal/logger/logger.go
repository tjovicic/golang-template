package logger

import (
	"github.com/rs/zerolog"
	"os"
)

func NewLogger(service, level string) (zerolog.Logger, error) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// user unix format for time attribute
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	logger = logger.With().Str("service", service).Logger()
	logger = logger.With().CallerWithSkipFrameCount(zerolog.CallerSkipFrameCount).Logger()

	zlLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return zerolog.Logger{}, err
	}

	logger = logger.Level(zlLevel)

	return logger, nil
}
