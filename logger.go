package gobase

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

var defaultLogger Logger = &DummyLogger{}

func LogErrorln(ss ...any) {
	s := fmt.Sprintln(ss...) + "\n"

	if defaultLogger == nil {
		os.Stderr.Write([]byte(s))
	} else {
		defaultLogger.Write([]byte(s))
	}
}

func LogErrorf(errf string, arg ...any) {
	s := fmt.Sprintf(errf, arg...)
	LogErrorln(s)
}

type Logger interface {
	io.Writer
	Close() error
	Message(level int32, kind string, message string, extras ...map[string]any) bool
	AddRequestID(requestUid string, fields ...map[string]any) Logger
	SetField(key string, value any)
	SetFields(map[string]any)
	SetAsDefault() Logger
}

type DummyLogger struct {
	Logger
}

func (DummyLogger) Close() error {
	return nil
}

func (DummyLogger) Message(level int32, kind string, message string, extras ...map[string]any) bool {
	if data, err := json.MarshalIndent(extras, "", "    "); err != nil {
		log.Println("WARN log not sent", level, kind, message)
	} else {
		log.Println("WARN log not sent", level, kind, message, string(data))
	}

	return true
}

func (dummy DummyLogger) AddRequestID(string, ...map[string]any) Logger {
	return dummy
}

func (DummyLogger) SetField(string, any) {
}

func (DummyLogger) SetFields(map[string]any) {
}

func (dummy DummyLogger) Write(p []byte) (int, error) {
	return 0, nil
}

func (dummy DummyLogger) SetAsDefault() Logger {
	return dummy
}
