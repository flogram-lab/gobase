package gobase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"dario.cat/mergo"
	"github.com/flogram-lab/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type LogServiceForwarder struct {
	Logger
	addr     string
	facility string
	conn     *grpc.ClientConn
	client   proto.LogServiceClient
	fields   map[string]any
}

func NewLogServiceForwarder(facility, addr string) Logger {
	creds, err := LoadTLSCredentials()
	if err != nil {
		err = errors.Wrap(err, "LogServiceForwarder: cannot load TLS credentials: %w")
		panic(err)
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		err = errors.Wrap(err, "LogServiceForwarder: cannot dial GRPC with credentials: %w")
		panic(err)
	}

	logger := &LogServiceForwarder{
		facility: facility,
		fields:   make(map[string]any),
		addr:     addr,
		conn:     conn,
		client:   proto.NewLogServiceClient(conn),
	}

	var v *proto.ServiceIdentity
	v, err = logger.client.Whois(context.TODO(), &emptypb.Empty{})
	if _ = v; err != nil {
		log.Fatalf("LogServiceForwarder: Whois: %s", err)
	}

	return logger
}

func (logger *LogServiceForwarder) Close() error {
	return logger.conn.Close()
}

func (logger *LogServiceForwarder) AddRequestID(requestUid string, fields ...map[string]any) Logger {
	if oldId, ok := logger.fields["request_uid"]; ok {
		requestUid = oldId.(string) + "/" + requestUid
	}

	newFields := map[string]any{}
	mergo.Merge(&newFields, logger.fields, mergo.WithOverride)

	for _, v := range fields {
		mergo.Merge(&newFields, v, mergo.WithOverride)
	}

	newFields["request_uid"] = requestUid

	return &LogServiceForwarder{
		facility: logger.facility,
		fields:   newFields,
		addr:     logger.addr,
		conn:     logger.conn,
		client:   logger.client,
	}
}

func (logger *LogServiceForwarder) SetField(key string, value any) {
	logger.fields[key] = value
}

func (logger *LogServiceForwarder) SetFields(newFields map[string]any) {
	mergo.Merge(&logger.fields, newFields, mergo.WithOverride)
}

func (logger *LogServiceForwarder) Message(level int32, kind string, message string, fields ...map[string]any) bool {

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

	messageFieldsStrings := make(map[string]string)
	for k, v := range messageFields {
		messageFieldsStrings[k] = fmt.Sprintf("%v", v)
	}

	m := &proto.LogMessage{
		Facility: logger.facility,
		Kind:     kind,
		Message:  message,
		Level:    level,
		Fields:   messageFieldsStrings,
	}

	var (
		err error
		v   *emptypb.Empty
	)
	v, err = logger.client.Message(context.TODO(), m)

	if _ = v; err != nil {
		log.Println("ERROR LogServiceForwarder.Message():", err.Error())

		if data, err := json.MarshalIndent(fields, "", "    "); err != nil {
			log.Println("WARN log not sent", err)
		} else {
			log.Println("WARN log not sent", string(data))
		}

		return false
	}

	return true
}

func (logger *LogServiceForwarder) Write(p []byte) (int, error) {
	if logger.Message(gelf.LOG_INFO, "stdout", strings.Trim(string(p), "\n ")) {
		return len(p), nil
	} else {
		return 0, errors.New("logger.Message() returned false")
	}
}

func (l *LogServiceForwarder) SetAsDefault() Logger {
	defaultLogger = l
	return l
}
