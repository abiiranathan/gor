package gor

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

// Wrapper around the standard http.Server.
// Adds easy graceful shutdown and functional options for customizing the server.
// This is the default server used by gor.
type Server struct {
	*http.Server
}

// Option for configuring the server.
type ServerOption func(*Server)

// Create a new Server instance.
func NewServer(addr string, handler http.Handler, options ...ServerOption) *Server {
	server := &Server{
		&http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  15 * time.Second,
		},
	}

	for _, option := range options {
		option(server)
	}
	return server
}

// Gracefully shuts down the server. The default timeout is 5 seconds
// To wait for pending connections.
func (s *Server) GracefulShutdown(timeout ...time.Duration) {
	var t time.Duration
	if len(timeout) > 0 {
		t = timeout[0]
	} else {
		t = 5 * time.Second
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		panic(err)
	}

	log.Println("shutting down")
}

// define a few options to configure the server
func WithReadTimeout(d time.Duration) ServerOption {
	return func(s *Server) {
		s.ReadTimeout = d
	}
}

func WithWriteTimeout(d time.Duration) ServerOption {
	return func(s *Server) {
		s.WriteTimeout = d
	}
}

func WithIdleTimeout(d time.Duration) ServerOption {
	return func(s *Server) {
		s.IdleTimeout = d
	}
}

func WithTLSConfig(config *tls.Config) ServerOption {
	return func(s *Server) {
		s.TLSConfig = config
	}
}
