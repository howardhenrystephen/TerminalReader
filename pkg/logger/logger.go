package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	TODO
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case TODO:
		return "TODO"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	mu     sync.Mutex
	logDir string
	writer io.Writer
	file   *os.File
	date   string
}

var (
	globalLogger *Logger
	once         sync.Once
)

func Init(logDir string) error {
	var initErr error
	once.Do(func() {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			initErr = fmt.Errorf("failed to create log directory: %w", err)
			return
		}
		globalLogger = &Logger{
			logDir: logDir,
		}
		if err := globalLogger.rotate(); err != nil {
			initErr = fmt.Errorf("failed to open log file: %w", err)
			return
		}
	})
	return initErr
}

func (l *Logger) rotate() error {
	now := time.Now()
	dateStr := now.Format("2006-01-02")

	if l.date == dateStr && l.file != nil {
		return nil
	}

	if l.file != nil {
		l.file.Close()
	}

	filename := filepath.Join(l.logDir, fmt.Sprintf("app_%s.log", dateStr))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	l.file = file
	l.date = dateStr
	l.writer = file
	return nil
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.rotate(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to rotate log file: %v\n", err)
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level.String(), msg)

	fmt.Fprint(l.writer, line)
}

func Debugf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.log(DEBUG, format, args...)
	}
}

func Infof(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.log(INFO, format, args...)
	}
}

func Warnf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.log(WARN, format, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.log(ERROR, format, args...)
	}
}

func Todof(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.log(TODO, format, args...)
	}
}
