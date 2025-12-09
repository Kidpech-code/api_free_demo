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
	Engine *gin.Engine
	Addr   string
	Logger *zap.Logger
}

// Run starts the server with graceful shutdown.
func (s *Server) Run(ctx context.Context) error {
	if s.Engine == nil {
		return fmt.Errorf("engine not configured")
	}
	srv := &http.Server{
		Addr:    s.Addr,
		Handler: s.Engine,
	}
	if s.Logger != nil {
		s.Logger.Info("http server listening", zap.String("addr", s.Addr))
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}
