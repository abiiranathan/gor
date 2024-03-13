package logger

import (
	"errors"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/abiiranathan/egor/egor"
)

// LogFormat is the format of the log output, compatible with the new slog package.
type LogFormat int

const (
	TextFormat LogFormat = iota // This is the default format
	JSONFormat                  // Log in JSON format
)

// LoggerMiddleware is a middleware that logs the request and response information.
type LoggerMiddleware struct {
	Output  io.Writer
	Format  LogFormat
	LogIP   bool
	Skip    []string
	Options *slog.HandlerOptions
}

// New creates a new LoggerMiddleware with the specified configuration.
func New(output io.Writer, skip ...string) *LoggerMiddleware {
	lm := &LoggerMiddleware{
		Output: output,
		Format: TextFormat,
		LogIP:  true,
		Skip:   skip,
		Options: &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: false,
		},
	}

	return lm
}

// Logger is the middleware handler function for LoggerMiddleware.
func (l *LoggerMiddleware) Logger(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var skipThis bool
		for _, s := range l.Skip {
			if s == req.URL.Path {
				skipThis = true
				break
			}
		}

		if skipThis {
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
			log.Printf("Unknown log format: %d. Using default format\n", l.Format)
			logger = slog.New(slog.NewTextHandler(l.Output, l.Options))
		}

		ipAddr, _ := getIP(req)

		if l.LogIP {
			logger.Info("", "status", w.(*egor.ResponseWriter).Status(), "latency", latency, "method", req.Method,
				"path", req.URL.Path, "ip", ipAddr)
		} else {
			logger.Info("", "status", w.(*egor.ResponseWriter).Status(), "latency", latency, "method", req.Method,
				"path", req.URL.Path)
		}
	})
}

func getIP(r *http.Request) (string, error) {
	ips := r.Header.Get("X-Forwarded-For")
	splitIps := strings.Split(ips, ",")

	if len(splitIps) > 0 {
		// get last IP in list since ELB prepends other user defined IPs,
		// meaning the last one is the actual client IP.
		netIP := net.ParseIP(splitIps[len(splitIps)-1])
		if netIP != nil {
			return netIP.String(), nil
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}

	netIP := net.ParseIP(ip)
	if netIP != nil {
		ip := netIP.String()
		if ip == "::1" {
			return "127.0.0.1", nil
		}
		return ip, nil
	}
	return "", errors.New("IP not found")
}
