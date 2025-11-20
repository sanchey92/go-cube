package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	client "github.com/sanchey92/go-cube/internal/client/container"
	workerhttp "github.com/sanchey92/go-cube/internal/http/worker"
	"github.com/sanchey92/go-cube/internal/services/worker"
)

const (
	defaultWorkerName = "worker-1"
	defaultHost       = "0.0.0.0"
	defaultPort       = 8080
	shutdownTimeout   = 30 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}

func run() error {
	// Create Docker client
	dockerClient, err := client.NewDocker()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	log.Println("Docker client created successfully")

	// Create Worker service
	worker := worker.New(defaultWorkerName, dockerClient)
	log.Printf("Worker '%s' initialized", defaultWorkerName)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background worker routines
	go worker.CollectStats(ctx)
	log.Println("Stats collection started")

	go worker.UpdateTasks(ctx)
	log.Println("Task updater started")

	go worker.RunTasks(ctx)
	log.Println("Task runner started")

	// Create HTTP server
	httpServer := workerhttp.NewHTTPServer(defaultHost, defaultPort, worker)

	// Start HTTP server in goroutine
	serverAddr := fmt.Sprintf("%s:%d", defaultHost, defaultPort)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      httpServer.GetRouter(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for errors from the HTTP server
	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("HTTP server starting on %s", serverAddr)
		serverErrors <- srv.ListenAndServe()
	}()

	// Channel to listen for interrupt signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-shutdown:
		log.Printf("Received signal %v, starting graceful shutdown", sig)

		// Cancel context to stop background workers
		cancel()

		// Give outstanding requests time to complete
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during server shutdown: %v", err)
			// Force close if graceful shutdown fails
			if closeErr := srv.Close(); closeErr != nil {
				return fmt.Errorf("forced server close failed: %w", closeErr)
			}
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		log.Println("Server stopped gracefully")
	}

	return nil
}
