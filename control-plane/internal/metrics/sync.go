package metrics

import (
	"context"

	"finalproject/control-plane/internal/ledger"
	"finalproject/control-plane/internal/model"
	"finalproject/control-plane/internal/orchestration"
)

// Refresh 从账本、任务库与集群刷新 gauge（在 /metrics 或后台 ticker 调用）。
func Refresh(ctx context.Context, lg *ledger.Ledger, store *orchestration.WorkloadStore, cluster *orchestration.Cluster) {
	if lg != nil {
		var n int64
		for _, a := range lg.ListAllocations() {
			if a != nil && a.Phase != model.PhaseReleased {
				n++
			}
		}
		SetAllocationsActive(n)
	}
	ResetWorkloadPhases()
	if store != nil {
		for _, ph := range []model.WorkloadPhase{
			model.WorkloadPhasePending,
			model.WorkloadPhaseRunning,
			model.WorkloadPhaseSucceeded,
			model.WorkloadPhaseFailed,
			model.WorkloadPhaseUnknown,
		} {
			list := store.List("", ph)
			SetWorkloadPhase(string(ph), int64(len(list)))
		}
	}
	if cluster != nil && cluster.Enabled() {
		if ctx == nil {
			ctx = context.Background()
		}
		m, err := cluster.PodPhaseCounts(ctx)
		if err == nil {
			SetK8sPodsPending(int64(m["Pending"]))
		}
	}
}
