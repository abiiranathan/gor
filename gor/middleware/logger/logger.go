package logger

import (
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/abiiranathan/gor/gor"
)

// LogFormat is the format of the log output, compatible with the new slog package.
type LogFormat int
type LogFlags int8

const (
	TextFormat LogFormat = iota // This is the default format
	JSONFormat                  // Log in JSON format
)

const (
	LOG_IP LogFlags = 1 << iota
	LOG_LATENCY
	LOG_USERAGENT
)

const StdLogFlags LogFlags = LOG_LATENCY | LOG_IP

// LoggerMiddleware is a middleware that logs the request and response information.
type LoggerMiddleware struct {
	Output  io.Writer
	Format  LogFormat
	Flags   LogFlags
	Skip    []string
	Options *slog.HandlerOptions

	// Callback is a function that can be used to modify the arguments passed to the logger.
	// Forexample the request_id, user_id etc.
	Callback func(args ...any) []any
}

// New creates a new LoggerMiddleware writing to output.
// Modify what is logged with a bit-mask of flags.
// You can also pass a callback function that can modify the arguments passed to the logger.
// If the callback is nil, the arguments are not modified. The args must be in key-value pairs.
// as required by the slog package.
// e.g args=append(args, "user_id", 100)
func New(output io.Writer, flags LogFlags, callback func(args ...any) []any, skip ...string) gor.Middleware {
	lm := &LoggerMiddleware{
		Output:   output,
		Format:   TextFormat,
		Flags:    flags,
		Skip:     skip,
		Callback: callback,
		Options: &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: false,
		},
	}

	return lm.Logger
}

// Logger is the middleware handler function for LoggerMiddleware.
func (l *LoggerMiddleware) Logger(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if slices.Contains(l.Skip, req.URL.Path) {
			handler.ServeHTTP(w, req)
			return
		}

		start := time.Now()
		handler.ServeHTTP(w, req)
		latency := time.Since(start).String()

		var logger *slog.Logger
		switch l.Format {
		case TextFormat:
			logger = slog.New(slog.NewTextHandler(l.Output, l.Options))
		case JSONFormat:
			logger = slog.New(slog.NewJSONHandler(l.Output, l.Options))
		default:
			logger = slog.New(slog.NewTextHandler(l.Output, l.Options))
		}

		args := []any{"status", w.(*gor.ResponseWriter).Status()}
		if l.Flags&LOG_LATENCY != 0 {
			args = append(args, "latency", latency)
		}
		args = append(args, "method", req.Method, "path", req.URL.Path)

		if l.Flags&LOG_IP != 0 {
			ipAddr, _ := gor.ClientIPAddress(req)
			args = append(args, "ip", ipAddr)
		}

		if l.Flags&LOG_USERAGENT != 0 {
			args = append(args, "user_agent", req.UserAgent())
		}

		if l.Callback != nil {
			args = l.Callback(args...)

			if len(args)%2 != 0 {
				panic("Callback must return an even number of arguments")
			}
		}

		logger.Info("", args...)
	})
}
