package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentation-service/segmentation/internal/application"
	"github.com/segmentation-service/segmentation/internal/domain/engine"
	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/ports"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
	"github.com/segmentation-service/segmentation/internal/domain/validation"
	infraConfig "github.com/segmentation-service/segmentation/internal/infrastructure/config"
	"github.com/segmentation-service/segmentation/internal/infrastructure/hash"
	infraHTTP "github.com/segmentation-service/segmentation/internal/infrastructure/http"
	"github.com/segmentation-service/segmentation/internal/infrastructure/search"
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
	snap, warnings, err := loadAndValidate(fileSource)
	if err != nil {
		log.Fatalf("%v", err)
	}
	for _, w := range warnings {
		log.Printf("[config] warning: %s", w)
	}
	memStore.Swap(snap)
	log.Printf("loaded config version %d with %d layers", snap.Version, len(snap.Layers))

	// Domain: strategies + evaluator
	// Keyed by the model's strategy constants so a rename cannot leave the map
	// pointing at a name the evaluator will never look up.
	strategies := map[string]strategy.Strategy{
		model.StrategyStatic:     &strategy.StaticStrategy{},
		model.StrategyRule:       &strategy.RuleStrategy{},
		model.StrategyPercentage: &strategy.PercentageStrategy{Hasher: hasher},
		model.StrategyChecklist:  &strategy.ChecklistStrategy{},
	}
	evaluator := engine.NewEvaluator(strategies)

	// Application: use cases
	evaluateUC := application.NewEvaluateUseCase(memStore, evaluator)
	batchUC := application.NewBatchEvaluateUseCase(evaluateUC)
	reloadUC := application.NewReloadUseCase(fileSource, memStore)
	adminUC := application.NewAdminUseCase(memStore, fileSource)
	// The one line that changes when the config moves to a database: swap the
	// snapshot scan for a store-backed Searcher. Nothing above or below it
	// knows which is in use.
	searchUC := application.NewSearchUseCase(search.NewSnapshotSearcher(memStore))

	// Config watcher
	watcher := infraConfig.NewWatcher(fileSource, memStore, *configPath, 500*time.Millisecond)
	watcher.Start()
	defer watcher.Stop()

	// HTTP server
	srv := infraHTTP.NewServer(*addr, evaluateUC, batchUC, reloadUC, adminUC, searchUC, memStore)

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

// loadAndValidate loads a snapshot from source and validates it before it can
// ever reach a store. The reload endpoint and the file watcher both already
// validate on every subsequent load; without this, the process's very first
// snapshot — the one most likely to be hand-edited, stale, or still carrying
// fields from before a config migration — would boot and serve with no
// checks run at all. It returns the advisory warnings from
// validation.WarnMissingInputSchemas alongside the snapshot: these are not
// errors and must not block startup, only be surfaced.
func loadAndValidate(source ports.ConfigSource) (*model.Snapshot, []string, error) {
	snap, err := source.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	if err := validation.ValidateSnapshot(snap); err != nil {
		return nil, nil, fmt.Errorf("config is invalid: %w", err)
	}
	return snap, validation.WarnMissingInputSchemas(snap), nil
}
