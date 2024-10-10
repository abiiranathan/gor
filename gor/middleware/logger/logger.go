package logger

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/abiiranathan/gor/gor"
)

// LogFormat is the format of the log output, compatible with the new slog package.
type LogFormat int
type LogFlags int8

const (
	TextFormat LogFormat = iota + 1 // This is the default format
	JSONFormat                      // Log in JSON format
)

const (
	LOG_IP LogFlags = 1 << iota
	LOG_LATENCY
	LOG_USERAGENT
)

const StdLogFlags LogFlags = LOG_LATENCY | LOG_IP

// Config is a middleware that logs the request and response information.
type Config struct {
	// Output is the destination for the log output. If nil, os.Stderr is used.
	Output io.Writer

	// Format is the format of the log output. Default is TextFormat.
	Format LogFormat

	// Flags is the flags to be used for logging. Default is StdLogFlags.
	Flags LogFlags

	// Skip is a slice of paths that should not be logged.
	Skip []string

	// SkipIf is a function that can be used to skip logging based on the request.
	// If it returns true, the request will not be logged.
	SkipIf func(r *http.Request) bool

	// Options is the options to be passed to the slog.Handler.
	Options *slog.HandlerOptions

	// Callback is a function that can be used to modify the arguments passed to the logger.
	// Forexample the request_id, user_id etc.
	Callback func(r *http.Request, args ...any) []any
}

// DefaultLogger is the default logger used by the Logger middleware.
// It writes logs to os.Stderr with the TextFormat and StdLogFlags.
// The log level is set to Info.
var DefaultLogger = &Config{
	Output: os.Stderr,
	Format: TextFormat,
	Flags:  StdLogFlags,
	Options: &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	},
}

func New(config *Config) gor.Middleware {
	if config == nil {
		config = DefaultLogger
	}

	if config.Output == nil {
		config.Output = os.Stderr
	}

	if config.Format == 0 {
		config.Format = TextFormat
	}

	if config.Options == nil {
		config.Options = &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: false,
		}
	}

	return config.Logger
}

// Logger is the middleware handler function for LoggerMiddleware.
func (l *Config) Logger(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if slices.Contains(l.Skip, req.URL.Path) {
			handler.ServeHTTP(w, req)
			return
		}

		if l.SkipIf != nil && l.SkipIf(req) {
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
			args = l.Callback(req, args...)

			if len(args)%2 != 0 {
				panic("Callback must return an even number of arguments")
			}
		}

		logger.Info("", args...)
	})
}
