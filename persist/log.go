package persist

import (
	"io"

	"gitlab.com/NebulousLabs/Sia/build"
	"gitlab.com/NebulousLabs/log"
)

// Logger is a wrapper for log.Logger.
type Logger struct {
	*log.Logger
}

var (
	// options contains log options with Sia- and build-specific information.
	options = log.Options{
		BinaryName:   "Sia",
		BugReportURL: build.IssuesURL,
		Debug:        build.DEBUG,
		Release:      build.Release,
		Version:      build.Version,
	}
)

// NewFileLogger returns a logger that logs to logFilename. The file is opened
// in append mode, and created if it does not exist.
func NewFileLogger(logFilename string) (*Logger, error) {
	logger, err := log.NewFileLogger(logFilename, options)
	return &Logger{logger}, err
}

// NewLogger returns a logger that can be closed. Calls should not be made to
// the logger after 'Close' has been called.
func NewLogger(w io.Writer) *Logger {
	return &Logger{log.NewLogger(w, options)}
}
