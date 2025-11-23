package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	containerClient "github.com/sanchey92/go-cube/internal/client/container"
	managerHTTP "github.com/sanchey92/go-cube/internal/http/manager"
	workerHTTP "github.com/sanchey92/go-cube/internal/http/worker"
	"github.com/sanchey92/go-cube/internal/services/manager"
	"github.com/sanchey92/go-cube/internal/services/scheduler"
	"github.com/sanchey92/go-cube/internal/services/worker"
	"github.com/sanchey92/go-cube/internal/store"
)

const (
	defaultManagerPort = 8080
	defaultWorkerPort  = 8081
	defaultHost        = "localhost"
	shutdownTimeout    = 30 * time.Second
)

var (
	mode        = flag.String("mode", "worker", "Application mode: manager, worker, or all")
	host        = flag.String("host", defaultHost, "Host address")
	port        = flag.Int("port", defaultWorkerPort, "Port number")
	managerPort = flag.Int("manager-port", defaultManagerPort, "Manager port (used in 'all' mode)")
	workers     = flag.String("workers", "", "Comma-separated list of worker addresses (e.g., localhost:8081,localhost:8082)")
	name        = flag.String("name", "", "Worker name (optional)")
)

func main() {
	flag.Parse()

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting go-cube application...")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// WaitGroup to track all goroutines
	var wg sync.WaitGroup

	// Channel to collect errors from goroutines
	errChan := make(chan error, 10)

	switch *mode {
	case "worker":
		runWorker(ctx, &wg, errChan)
	case "manager":
		runManager(ctx, &wg, errChan)
	case "all":
		runAll(ctx, &wg, errChan)
	default:
		log.Fatalf("Invalid mode: %s. Use 'worker', 'manager', or 'all'", *mode)
	}

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)
	case err := <-errChan:
		log.Printf("Fatal error occurred: %v. Shutting down...", err)
	}

	// Cancel context to stop all goroutines
	cancel()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All services stopped gracefully")
	case <-time.After(shutdownTimeout):
		log.Println("Shutdown timeout exceeded, forcing exit")
	}

	log.Println("Application shutdown complete")
}

func runWorker(ctx context.Context, wg *sync.WaitGroup, errChan chan<- error) {
	log.Printf("Starting worker on %s:%d", *host, *port)

	// Create Docker client
	dockerClient, err := containerClient.NewDocker()
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	// Create worker name if not provided
	workerName := *name
	if workerName == "" {
		workerName = fmt.Sprintf("worker-%s:%d", *host, *port)
	}

	// Create worker instance
	w := worker.New(workerName, dockerClient)

	// Start worker background tasks
	wg.Add(3)
	go func() {
		defer wg.Done()
		w.RunTasks(ctx)
	}()
	go func() {
		defer wg.Done()
		w.UpdateTasks(ctx)
	}()
	go func() {
		defer wg.Done()
		w.CollectStats(ctx)
	}()

	// Create and start HTTP server
	srv := workerHTTP.NewHTTPServer(*host, *port, w)
	startHTTPServer(ctx, wg, errChan, srv.GetRouter(), *port, "worker")
}

func runManager(ctx context.Context, wg *sync.WaitGroup, errChan chan<- error) {
	log.Printf("Starting manager on %s:%d", *host, *port)

	// Parse worker addresses
	workerAddrs := parseWorkers(*workers)
	if len(workerAddrs) == 0 {
		log.Fatal("No workers specified. Use -workers flag to provide comma-separated worker addresses")
	}

	// Create stores
	taskStore := store.NewInMemoryStore()
	eventStore := store.NewInMemoryStore()

	// Create scheduler
	sched := &scheduler.RoundRobin{Name: "round-robin"}

	// Create manager instance
	mgr, err := manager.New(workerAddrs, taskStore, eventStore, sched)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	// Start manager background tasks
	wg.Add(3)
	go func() {
		defer wg.Done()
		mgr.ProcessTasks(ctx)
	}()
	go func() {
		defer wg.Done()
		mgr.UpdateTasks(ctx)
	}()
	go func() {
		defer wg.Done()
		mgr.DoHealthChecks(ctx)
	}()

	// Create and start HTTP server
	srv := managerHTTP.NewHTTPServer(*host, *port, mgr)
	startHTTPServer(ctx, wg, errChan, srv.GetRouter(), *port, "manager")
}

func runAll(ctx context.Context, wg *sync.WaitGroup, errChan chan<- error) {
	log.Println("Starting in 'all' mode: manager + worker")

	// Start worker
	workerHost := *host
	workerPort := *port
	workerAddr := fmt.Sprintf("%s:%d", workerHost, workerPort)

	// Create Docker client for worker
	dockerClient, err := containerClient.NewDocker()
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	// Create worker
	workerName := *name
	if workerName == "" {
		workerName = fmt.Sprintf("worker-%s", workerAddr)
	}
	w := worker.New(workerName, dockerClient)

	// Start worker background tasks
	wg.Add(3)
	go func() {
		defer wg.Done()
		w.RunTasks(ctx)
	}()
	go func() {
		defer wg.Done()
		w.UpdateTasks(ctx)
	}()
	go func() {
		defer wg.Done()
		w.CollectStats(ctx)
	}()

	// Start worker HTTP server
	workerSrv := workerHTTP.NewHTTPServer(workerHost, workerPort, w)
	startHTTPServer(ctx, wg, errChan, workerSrv.GetRouter(), workerPort, "worker")

	// Create manager
	managerHost := *host
	managerPortNum := *managerPort
	workerAddrs := []string{workerAddr}

	// Add additional workers if specified
	if *workers != "" {
		additionalWorkers := parseWorkers(*workers)
		workerAddrs = append(workerAddrs, additionalWorkers...)
	}

	// Create stores
	taskStore := store.NewInMemoryStore()
	eventStore := store.NewInMemoryStore()

	// Create scheduler
	sched := &scheduler.RoundRobin{Name: "round-robin"}

	// Create manager instance
	mgr, err := manager.New(workerAddrs, taskStore, eventStore, sched)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	// Start manager background tasks
	wg.Add(3)
	go func() {
		defer wg.Done()
		mgr.ProcessTasks(ctx)
	}()
	go func() {
		defer wg.Done()
		mgr.UpdateTasks(ctx)
	}()
	go func() {
		defer wg.Done()
		mgr.DoHealthChecks(ctx)
	}()

	// Start manager HTTP server
	managerSrv := managerHTTP.NewHTTPServer(managerHost, managerPortNum, mgr)
	startHTTPServer(ctx, wg, errChan, managerSrv.GetRouter(), managerPortNum, "manager")
}

func startHTTPServer(ctx context.Context, wg *sync.WaitGroup, errChan chan<- error, router http.Handler, port int, name string) {
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("Starting %s HTTP server on port %d", name, port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("%s server error: %w", name, err)
		}
	}()

	// Shutdown handler
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		log.Printf("Shutting down %s HTTP server on port %d...", name, port)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during %s server shutdown: %v", name, err)
		} else {
			log.Printf("%s HTTP server stopped", name)
		}
	}()
}

func parseWorkers(workersList string) []string {
	if workersList == "" {
		return nil
	}

	workers := strings.Split(workersList, ",")
	result := make([]string, 0, len(workers))

	for _, w := range workers {
		trimmed := strings.TrimSpace(w)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
