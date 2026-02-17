package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Server wraps the HTTP server for graceful lifecycle.
type Server struct {
	Engine          *gin.Engine
	Addr            string
	Logger          *zap.Logger
	ShutdownTimeout time.Duration // defaults to 15s
	ReadTimeout     time.Duration // defaults to 10s
	WriteTimeout    time.Duration // defaults to 30s
	IdleTimeout     time.Duration // defaults to 120s
}

// Run starts the server with production-grade graceful shutdown.
// Implements connection draining to finish in-flight requests before stopping.
func (s *Server) Run(ctx context.Context) error {
	if s.Engine == nil {
		return fmt.Errorf("engine not configured")
	}

	if s.ShutdownTimeout == 0 {
		s.ShutdownTimeout = 15 * time.Second
	}
	if s.ReadTimeout == 0 {
		s.ReadTimeout = 10 * time.Second
	}
	if s.WriteTimeout == 0 {
		s.WriteTimeout = 30 * time.Second
	}
	if s.IdleTimeout == 0 {
		s.IdleTimeout = 120 * time.Second
	}

	srv := &http.Server{
		Addr:         s.Addr,
		Handler:      s.Engine,
		ReadTimeout:  s.ReadTimeout,
		WriteTimeout: s.WriteTimeout,
		IdleTimeout:  s.IdleTimeout,
	}

	if s.Logger != nil {
		s.Logger.Info("http server listening",
			zap.String("addr", s.Addr),
			zap.Duration("read_timeout", s.ReadTimeout),
			zap.Duration("write_timeout", s.WriteTimeout),
			zap.Duration("idle_timeout", s.IdleTimeout),
		)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		if s.Logger != nil {
			s.Logger.Info("graceful shutdown initiated — draining connections...",
				zap.Duration("timeout", s.ShutdownTimeout))
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.ShutdownTimeout)
		defer cancel()

		// Graceful shutdown: stops accepting new connections, waits for in-flight to finish
		if err := srv.Shutdown(shutdownCtx); err != nil {
			if s.Logger != nil {
				s.Logger.Error("graceful shutdown failed — forcing close", zap.Error(err))
			}
			return srv.Close()
		}

		if s.Logger != nil {
			s.Logger.Info("graceful shutdown completed — all connections drained")
		}
		return nil

	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}
