package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentation-service/segmentation/internal/application"
	"github.com/segmentation-service/segmentation/internal/domain/engine"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
	infraConfig "github.com/segmentation-service/segmentation/internal/infrastructure/config"
	"github.com/segmentation-service/segmentation/internal/infrastructure/hash"
	infraHTTP "github.com/segmentation-service/segmentation/internal/infrastructure/http"
	"github.com/segmentation-service/segmentation/internal/infrastructure/store"
)

func main() {
	configPath := flag.String("config", "config/segments.json", "path to segments config file")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	// Infrastructure
	hasher := &hash.FNV{}
	memStore := store.NewMemory()
	fileSource := infraConfig.NewFileSource(*configPath)

	// Initial load
	snap, err := fileSource.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	memStore.Swap(snap)
	log.Printf("loaded config version %d with %d layers", snap.Version, len(snap.Layers))

	// Domain: strategies + evaluator
	strategies := map[string]strategy.Strategy{
		"static":     &strategy.StaticStrategy{},
		"rule":       &strategy.RuleStrategy{},
		"percentage": &strategy.PercentageStrategy{Hasher: hasher},
	}
	evaluator := engine.NewEvaluator(strategies)

	// Application: use cases
	evaluateUC := application.NewEvaluateUseCase(memStore, evaluator)
	batchUC := application.NewBatchEvaluateUseCase(evaluateUC)
	reloadUC := application.NewReloadUseCase(fileSource, memStore)

	// Config watcher
	watcher := infraConfig.NewWatcher(fileSource, memStore, *configPath, 500*time.Millisecond)
	watcher.Start()
	defer watcher.Stop()

	// HTTP server
	srv := infraHTTP.NewServer(*addr, evaluateUC, batchUC, reloadUC, memStore)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("listening on %s", *addr)
	if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("server error: %v", err)
	}
}
