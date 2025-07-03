package logging

import (
	"io"
	"log/slog"
	"os"
)

// NewLogger should only be used in main function, and must defer sync.
func NewLogger(logFilename string) (*slog.Logger, *os.File) {
	// create log file
	fd, err := os.Create(logFilename)
	if err != nil {
		// directly panic
		panic(err)
	}
	imw := io.MultiWriter(os.Stdout, fd)

	defaultLevel := slog.LevelInfo
	if data, ok := os.LookupEnv("RunEnv"); ok {
		if data == "DEBUG" {
			// set debug level log
			defaultLevel = slog.LevelDebug
		}
	}
	jsonH := slog.NewJSONHandler(imw, &slog.HandlerOptions{
		AddSource: true,
		Level:     defaultLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)
				levelLabel, exists := LevelNames[level]
				if !exists {
					levelLabel = level.String()
				}
				a.Value = slog.StringValue(levelLabel)
			}
			return a
		},
	})
	logger := slog.New(jsonH)
	slog.SetDefault(logger)

	return logger, fd
}

const (
	LevelTrace = slog.Level(-8)
	LevelFatal = slog.Level(12)
)

var LevelNames = map[slog.Leveler]string{
	LevelTrace: "TRACE",
	LevelFatal: "FATAL",
}
