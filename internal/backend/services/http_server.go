// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package services contains HTTP servers.
package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/xcontext"
)

// NewFromConfig creates a new Server from the config.Service.
func NewFromConfig(service config.HTTPService, handler http.Handler) *Server {
	var cert *certData

	if service.IsSecure() {
		cert = &certData{
			certFile: service.GetCertFile(),
			keyFile:  service.GetKeyFile(),
		}
	}

	return &Server{
		server: &http.Server{
			Addr:    service.GetBindEndpoint(),
			Handler: handler,
		},
		certData: cert,
	}
}

// NewInsecure creates a new Server.
func NewInsecure(endpoint string, handler http.Handler) *Server {
	return &Server{
		server: &http.Server{
			Addr:    endpoint,
			Handler: handler,
		},
	}
}

// Server is the HTTP server.
type Server struct {
	server   *http.Server
	certData *certData
}

type certData struct {
	cert     tls.Certificate
	certFile string
	keyFile  string
	mu       sync.Mutex
	loaded   bool
}

func (c *certData) load() error {
	cert, err := tls.LoadX509KeyPair(c.certFile, c.keyFile)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.loaded = true
	c.cert = cert

	return nil
}

func (c *certData) getCert() (*tls.Certificate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.loaded {
		return nil, fmt.Errorf("the cert wasn't loaded yet")
	}

	return &c.cert, nil
}

func (c *certData) runWatcher(ctx context.Context, logger *zap.Logger) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("error creating fsnotify watcher: %w", err)
	}
	defer w.Close() //nolint:errcheck

	if err = w.Add(c.certFile); err != nil {
		return fmt.Errorf("error adding watch for file %s: %w", c.certFile, err)
	}

	if err = w.Add(c.keyFile); err != nil {
		return fmt.Errorf("error adding watch for file %s: %w", c.keyFile, err)
	}

	handleEvent := func(e fsnotify.Event) error {
		defer func() {
			if err = c.load(); err != nil {
				logger.Error("failed to load certs", zap.Error(err))

				return
			}

			logger.Info("reloaded certs")
		}()

		if !e.Has(fsnotify.Remove) && !e.Has(fsnotify.Rename) {
			return nil
		}

		if err = w.Remove(e.Name); err != nil {
			logger.Error("failed to remove file watch, it may have been deleted", zap.String("file", e.Name), zap.Error(err))
		}

		if err = w.Add(e.Name); err != nil {
			return fmt.Errorf("error adding watch for file %s: %w", e.Name, err)
		}

		return nil
	}

	for {
		select {
		case e := <-w.Events:
			if err = handleEvent(e); err != nil {
				return err
			}
		case err = <-w.Errors:
			return fmt.Errorf("received fsnotify error: %w", err)
		case <-ctx.Done():
			return nil
		}
	}
}

// Run the server.
func (s *Server) Run(ctx context.Context, logger *zap.Logger) error {
	logger.Info("server starting", zap.Bool("secure", s.certData != nil))
	defer logger.Info("server stopped")

	stop := xcontext.AfterFuncSync(ctx, func() { //nolint:contextcheck
		logger.Info("server stopping")

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCtxCancel()

		err := s.shutdown(shutdownCtx)
		if err != nil {
			logger.Error("failed to gracefully stop server", zap.Error(err))
		}
	})

	defer stop()

	if err := s.listenAndServe(ctx, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("failed to serve", zap.Error(err))

		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

func (s *Server) listenAndServe(ctx context.Context, logger *zap.Logger) error {
	if s.certData == nil {
		return s.server.ListenAndServe()
	}

	if err := s.certData.load(); err != nil {
		return fmt.Errorf("failed to load certs: %w", err)
	}

	s.server.TLSConfig = &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return s.certData.getCert()
		},
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg := panichandler.NewErrGroup()

	eg.Go(func() error {
		for {
			err := s.certData.runWatcher(ctx, logger)

			if err == nil {
				return nil
			}

			logger.Error("cert watcher crashed, restarting in 5 seconds", zap.Error(err))

			time.Sleep(time.Second * 5)
		}
	})

	eg.Go(func() error {
		defer cancel()

		return s.server.ListenAndServeTLS("", "")
	})

	return eg.Wait()
}

func (s *Server) shutdown(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	if !errors.Is(ctx.Err(), err) {
		return err
	}

	if closeErr := s.server.Close(); closeErr != nil {
		return fmt.Errorf("failed to close server: %w", closeErr)
	}

	return err
}
