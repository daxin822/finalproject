package orchestration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"finalproject/control-plane/internal/model"
)

// OrchestratorService 串联 render → apply → Store 注册。
type OrchestratorService struct {
	Cluster *Cluster
	Store   *WorkloadStore
}

func NewOrchestratorService(cluster *Cluster, store *WorkloadStore) *OrchestratorService {
	return &OrchestratorService{Cluster: cluster, Store: store}
}

func (o *OrchestratorService) SubmitTraining(ctx context.Context, spec TrainingWorkloadSpec) (*model.WorkloadRecord, error) {
	if o.Store == nil {
		return nil, errors.New("workload store not configured")
	}
	if o.Cluster == nil || !o.Cluster.Enabled() {
		return nil, errors.New("kubernetes client not configured")
	}
	yml, err := RenderTrainingYAML(spec)
	if err != nil {
		return nil, err
	}
	refs, err := o.Cluster.ApplyYAML(ctx, yml)
	if err != nil {
		return nil, err
	}
	rec := &model.WorkloadRecord{
		ID:               newWorkloadID("train"),
		Tenant:           spec.Tenant,
		Namespace:        spec.Namespace,
		Name:             spec.Name,
		Kind:             model.WorkloadKindTraining,
		Phase:            model.WorkloadPhasePending,
		FlavorID:         spec.FlavorID,
		ExtendedResource: spec.ExtendedRes,
		K8sRefs:          refs,
	}
	o.Store.Create(rec)
	out, ok := o.Store.Get(rec.ID)
	if !ok {
		return nil, fmt.Errorf("workload record lost after create")
	}
	return out, nil
}

func (o *OrchestratorService) SubmitInference(ctx context.Context, spec InferenceWorkloadSpec) (*model.WorkloadRecord, error) {
	if o.Store == nil {
		return nil, errors.New("workload store not configured")
	}
	if o.Cluster == nil || !o.Cluster.Enabled() {
		return nil, errors.New("kubernetes client not configured")
	}
	yml, err := RenderInferenceYAML(spec)
	if err != nil {
		return nil, err
	}
	refs, err := o.Cluster.ApplyYAML(ctx, yml)
	if err != nil {
		return nil, err
	}
	rec := &model.WorkloadRecord{
		ID:               newWorkloadID("infer"),
		Tenant:           spec.Tenant,
		Namespace:        spec.Namespace,
		Name:             spec.Name,
		Kind:             model.WorkloadKindInference,
		Phase:            model.WorkloadPhasePending,
		FlavorID:         spec.FlavorID,
		ExtendedResource: spec.ExtendedRes,
		K8sRefs:          refs,
	}
	o.Store.Create(rec)
	out, ok := o.Store.Get(rec.ID)
	if !ok {
		return nil, fmt.Errorf("workload record lost after create")
	}
	return out, nil
}

func (o *OrchestratorService) DeleteWorkload(ctx context.Context, id string) error {
	if o.Store == nil {
		return errors.New("workload store not configured")
	}
	rec, ok := o.Store.Get(id)
	if !ok {
		return fmt.Errorf("workload %s not found", id)
	}
	if o.Cluster != nil && o.Cluster.Enabled() && len(rec.K8sRefs) > 0 {
		refs := append([]string(nil), rec.K8sRefs...)
		for i := len(refs) - 1; i >= 0; i-- {
			if err := o.Cluster.DeleteRef(ctx, refs[i]); err != nil {
				return err
			}
		}
	}
	o.Store.Delete(id)
	return nil
}

func newWorkloadID(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().Unix(), hex.EncodeToString(b))
}
