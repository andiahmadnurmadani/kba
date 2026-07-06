package logs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"kroombox-backup-agent/modules"
	"time"
)

type Logger struct {
	info  *logWriter
	error *logWriter
	file  *os.File
}

type logWriter struct {
	prefix string
	w      io.Writer
}

func (lw *logWriter) Write(p []byte) (int, error) {
	// Use timezone from config, fallback to Asia/Jakarta (WIB)
	tz := os.Getenv("KBA_TZ")
	if tz == "" {
		tz = "Asia/Jakarta"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	now := time.Now().In(loc)
	ts := now.Format("2006/01/02 15:04:05")
	msg := fmt.Sprintf("%s%s: %s", lw.prefix, ts, string(p))
	return lw.w.Write([]byte(msg))
}

var defaultLogger *Logger

func Init(logDir string) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	path := filepath.Join(logDir, fmt.Sprintf("backup_%s.log", modules.NowInTZ().Format("20060102")))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	multi := io.MultiWriter(f)  // file only — stdout reserved for progress animation

	defaultLogger = &Logger{
		info:  &logWriter{prefix: "INFO: ", w: multi},
		error: &logWriter{prefix: "ERROR: ", w: multi},
		file:  f,
	}
	return nil
}

func Info(format string, args ...interface{}) {
	if defaultLogger != nil && defaultLogger.info != nil {
		msg := fmt.Sprintf(format, args...)
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		defaultLogger.info.Write([]byte(msg))
	}
}

func Error(format string, args ...interface{}) {
	if defaultLogger != nil && defaultLogger.error != nil {
		msg := fmt.Sprintf(format, args...)
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		defaultLogger.error.Write([]byte(msg))
	}
}

func Close() {
	if defaultLogger != nil && defaultLogger.file != nil {
		defaultLogger.file.Close()
	}
}

