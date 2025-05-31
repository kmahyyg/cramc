package logging

import (
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"time"
)

const logFilename = "cramc_go.log"

// NewLogger should only be used in main function, and must defer sync.
func NewLogger() (*logrus.Logger, *os.File) {
	logger := logrus.New()
	logger.Formatter = &logrus.TextFormatter{
		DisableColors:   true,
		ForceQuote:      true,
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
	}
	logger.SetReportCaller(true)
	logger.SetLevel(logrus.InfoLevel)
	if data, ok := os.LookupEnv("RunEnv"); ok {
		if data == "DEBUG" {
			// set debug level log
			logger.SetLevel(logrus.DebugLevel)
		}
	}
	fd, err := os.Create(logFilename)
	if err != nil {
		// directly panic
		panic(err)
	}
	logger.SetOutput(io.MultiWriter(os.Stdout, fd))
	return logger, fd
}
