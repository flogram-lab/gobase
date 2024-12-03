package gobase

import (
	"fmt"
	"log"
	"os"
	"time"

	"dario.cat/mergo"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type GelfLogger struct {
	writer   gelf.Writer
	hostname string
	fields   map[string]any
}

func NewGelfLogger(graylogAddr, selfHostname string) *GelfLogger {

	gelfWriter, err := gelf.NewTCPWriter(graylogAddr)
	if err != nil {
		log.Fatalf("gelf.NewTCPWriter: %s", err)
	}

	gelfWriter.Facility = "flo_log"

	logger := &GelfLogger{
		writer:   gelfWriter,
		hostname: selfHostname,
		fields:   map[string]any{},
	}

	log.Printf("Logging errors to stderr, full logging to  graylog @%s", graylogAddr)

	return logger
}

func (logger *GelfLogger) Close() error {
	return logger.writer.Close()
}

func (logger *GelfLogger) Message(facility string, level int32, kind string, message string, fields ...map[string]string) bool {

	messageFields := logger.fields

	if len(fields) > 0 {
		messageFields = make(map[string]any)

		mergo.Merge(&messageFields, logger.fields, mergo.WithOverride)

		for _, callExtraFields := range fields {
			mergo.Merge(&messageFields, callExtraFields, mergo.WithOverride)
		}
	}

	m := &gelf.Message{
		Version:  "1.1",
		Host:     logger.hostname,
		Short:    kind,
		Full:     message,
		TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
		Level:    level,
		Extra:    messageFields,
		Facility: facility,
	}

	err := logger.writer.WriteMessage(m)

	if err != nil {
		stdErrMessage := fmt.Sprintf("ERR message failed to log via GELF, err: %s, message: \"%s\" %s\n", err, kind, message)

		if ruid, ok := logger.fields["request_uid"].(string); ok && ruid != "" {
			stdErrMessage = fmt.Sprintf("[%s] %s", ruid, stdErrMessage)
		}

		os.Stderr.WriteString(stdErrMessage)

		return false
	}

	return true
}
