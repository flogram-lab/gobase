package gobase

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"dario.cat/mergo"
	"github.com/go-faster/errors"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type GelfLogger struct {
	Logger
	writer             gelf.Writer
	facility, hostname string
	fields             map[string]any
	stderr             bool
}

func NewGelfLogger(facility, graylogAddr, selfHostname string) Logger {

	gelfWriter, err := gelf.NewTCPWriter(graylogAddr)
	if err != nil {
		log.Fatalf("gelf.NewTCPWriter: %s", err)
	}

	gelfWriter.Facility = facility

	logger := &GelfLogger{
		writer:   gelfWriter,
		facility: facility,
		hostname: selfHostname,
		stderr:   true,
		fields:   map[string]any{},
	}

	log.Printf("Logging errors to stderr, full logging to  graylog @%s", graylogAddr)

	return logger
}

func (logger *GelfLogger) Close() error {
	return logger.writer.Close()
}

func (logger *GelfLogger) AddRequestID(requestUid string, fields ...map[string]any) Logger {
	if oldId, ok := logger.fields["request_uid"]; ok {
		requestUid = oldId.(string) + "/" + requestUid
	}

	newFields := map[string]any{}
	mergo.Merge(&newFields, logger.fields, mergo.WithOverride)

	for _, v := range fields {
		mergo.Merge(&newFields, v, mergo.WithOverride)
	}

	newFields["request_uid"] = requestUid

	return &GelfLogger{
		writer:   logger.writer,
		facility: logger.facility,
		hostname: logger.hostname,
		stderr:   logger.stderr,
		fields:   newFields,
	}
}

func (logger *GelfLogger) SetField(key string, value any) {
	logger.fields[key] = value
}

func (logger *GelfLogger) SetFields(newFields map[string]any) {
	mergo.Merge(&logger.fields, newFields, mergo.WithOverride)
}

func (logger *GelfLogger) Message(level int32, kind string, message string, fields ...map[string]any) bool {

	messageFields := logger.fields

	if len(fields) > 0 {
		messageFields = make(map[string]any)

		mergo.Merge(&messageFields, logger.fields, mergo.WithOverride)

		for _, callExtraFields := range fields {
			mergo.Merge(&messageFields, callExtraFields, mergo.WithOverride)
		}
	}

	if level <= gelf.LOG_ERR {
		stdErrMessage := fmt.Sprintf("%s: %s\n", kind, message)

		if ruid, ok := logger.fields["request_uid"].(string); ok && ruid != "" {
			stdErrMessage = fmt.Sprintf("[%s] %s", ruid, stdErrMessage)
		}

		os.Stderr.WriteString(stdErrMessage)
	}

	m := &gelf.Message{
		Version:  "1.1",
		Host:     logger.hostname,
		Short:    kind,
		Full:     message,
		TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
		Level:    level,
		Extra:    messageFields,
		Facility: logger.facility,
	}

	err := logger.writer.WriteMessage(m)
	if err == nil {
		return true
	}

	log.Println("ERROR WriteMessage GELF in GelfWriterLogging.Message:", err.Error())
	if data, err := json.MarshalIndent(fields, "", "    "); err != nil {
		log.Println("WARN log not sent", err)
	} else {
		log.Println("WARN log not sent", string(data))
	}

	return false
}

func (logger *GelfLogger) Write(p []byte) (int, error) {
	if logger.Message(gelf.LOG_INFO, "stdout", strings.Trim(string(p), "\n ")) {
		return len(p), nil
	} else {
		return 0, errors.New("logger.Message() returned false")
	}
}

func (l *GelfLogger) SetAsDefault() Logger {
	defaultLogger = l
	return l
}
