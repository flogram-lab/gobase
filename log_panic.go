package gobase

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/go-faster/errors"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

func LogPanic(logger Logger, kind string) {
	if r := recover(); r != nil {
		rs := fmt.Sprintf("recovered from panic: %s", r)
		ss := fmt.Sprintf("stacktrace from panic: \n%s", debug.Stack())
		fmt.Println(rs)
		fmt.Println(ss)
		if logger != nil {
			logger.Message(gelf.LOG_CRIT, kind, "panic (err, stacktrace)", map[string]any{
				"err":        rs,
				"stacktrace": ss,
			})
		}
	}
}

func LogPanicErr(errOut *error, logger Logger, kind string, errorTitle string) {
	if r := recover(); r != nil {
		*errOut = errors.New("panic (details hidden): " + errorTitle)
		rs := fmt.Sprintf("recovered from panic: %s", r)
		ss := fmt.Sprintf("stacktrace from panic: \n%s", debug.Stack())
		fmt.Println(rs)
		fmt.Println(ss)
		if logger != nil {
			logger.Message(gelf.LOG_CRIT, kind, "panic (err, stacktrace): "+errorTitle, map[string]any{
				"err":        rs,
				"stacktrace": ss,
			})
		}
	}
}

func LogPanicExit(logger Logger, kind string) {
	if r := recover(); r != nil {
		rs := fmt.Sprintf("recovered from panic (exiting): %s", r)
		ss := fmt.Sprintf("stacktrace from panic: \n%s", debug.Stack())
		fmt.Println(rs)
		fmt.Println(ss)
		if logger != nil {
			logger.Message(gelf.LOG_CRIT, kind, "panic (err, stacktrace)", map[string]any{
				"err":        rs,
				"stacktrace": ss,
			})
		}
		time.Sleep(time.Second * 5)
		os.Exit(1)
	}
}
