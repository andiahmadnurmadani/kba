package logs

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Logger struct {
	info  *log.Logger
	error *log.Logger
	file  *os.File
}

var defaultLogger *Logger

func Init(logDir string) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	path := filepath.Join(logDir, fmt.Sprintf("backup_%s.log", time.Now().Format("20060102")))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	multi := io.MultiWriter(f)  // file only — stdout reserved for progress animation

	defaultLogger = &Logger{
		info:  log.New(multi, "INFO: ", log.Ldate|log.Ltime),
		error: log.New(multi, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		file:  f,
	}
	return nil
}

func Info(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.info.Printf(format, args...)
	}
}

func Error(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.error.Printf(format, args...)
	}
}

func Close() {
	if defaultLogger != nil && defaultLogger.file != nil {
		defaultLogger.file.Close()
	}
}

