package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"finalproject/control-plane/internal/api"
	"finalproject/control-plane/internal/audit"
	"finalproject/control-plane/internal/auth"
	"finalproject/control-plane/internal/catalog"
	"finalproject/control-plane/internal/ledger"
	"finalproject/control-plane/internal/metrics"
	"finalproject/control-plane/internal/orchestration"
)

func main() {
	root, err := catalog.LoadEmbedded()
	if err != nil {
		log.Fatalf("catalog: %v", err)
	}
	l := ledger.New(root)
	kc, err := orchestration.TryConnect()
	if err != nil {
		log.Fatalf("kubernetes: %v", err)
	}
	if kc == nil {
		log.Printf("kubernetes: disabled (no kubeconfig); training/inference apply and pod watch unavailable")
	}
	store := orchestration.NewWorkloadStore()
	orch := orchestration.NewOrchestratorService(kc, store)

	authCfg := auth.LoadConfigFromEnv()
	if authCfg.Enabled() && len(authCfg.Secret) < 8 {
		log.Fatal("set CP_JWT_SECRET (>=8 bytes) to enable auth, or leave it unset for dev (auth off)")
	}

	srv := &api.Server{
		Ledger:       l,
		Cluster:      kc,
		Workloads:    store,
		Orchestrator: orch,
		Auth:         authCfg,
		Audit:        audit.New(500),
		Idempo:       api.NewIdempotencyStore(10 * time.Minute),
	}
	api.SetMetricsSnapshotHook(func(ctx context.Context) {
		metrics.Refresh(ctx, l, store, kc)
	})

	if kc != nil && kc.Enabled() {
		syncer := orchestration.NewStatusSyncer(kc, store, 10*time.Second)
		go syncer.Run(context.Background())
	}

	inner := http.NewServeMux()
	srv.Register(inner)
	handler := srv.Wrap(inner)

	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	log.Printf("control-plane listening on %s (GET /api/v1/openapi.yaml for contract)", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
