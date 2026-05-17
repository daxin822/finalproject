package ledger

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"finalproject/control-plane/internal/catalog"
	"finalproject/control-plane/internal/model"
)

var (
	ErrUnknownFlavor   = errors.New("unknown flavor")
	ErrUnknownPool     = errors.New("unknown pool")
	ErrFlavorNotInPool = errors.New("flavor not supported in pool")
	ErrNoCapacity      = errors.New("no free slice unit for flavor in pool")
	ErrUnknownAlloc    = errors.New("unknown allocation")
)

type Ledger struct {
	mu           sync.RWMutex
	cat          *catalog.Root
	nodes        []model.Node
	allocations  map[string]*model.Allocation
	nextAllocSeq int
}

func New(cat *catalog.Root) *Ledger {
	l := &Ledger{
		cat:         cat,
		allocations: make(map[string]*model.Allocation),
	}
	l.seedDemoInventory()
	return l
}

func (l *Ledger) seedDemoInventory() {
	l.nodes = []model.Node{
		{
			Name: "node-inf-01",
			Labels: map[string]string{
				"pool": "inference-310",
				"accelerator": "ascend-310",
			},
			PoolID: "pool-inference-310",
			Cards: []model.AcceleratorCard{
				{ID: "card-node-inf-01-0", NodeName: "node-inf-01", ChipModel: "Ascend310", SlotIndex: 0, PhysicalNPUs: 1},
			},
			Slices: []model.SliceUnit{
				{ID: "slice-inf-01-a", CardID: "card-node-inf-01-0", FlavorID: "ascend-310-vir02", Available: true},
				{ID: "slice-inf-01-b", CardID: "card-node-inf-01-0", FlavorID: "ascend-310-vir02", Available: true},
				{ID: "slice-inf-01-c", CardID: "card-node-inf-01-0", FlavorID: "ascend-310-vir04", Available: true},
			},
		},
		{
			Name: "node-train-01",
			Labels: map[string]string{
				"pool": "training-910b2",
				"accelerator": "Ascend910B2",
			},
			PoolID: "pool-training-910b2",
			Cards: []model.AcceleratorCard{
				{ID: "card-node-train-01-0", NodeName: "node-train-01", ChipModel: "Ascend910B2", SlotIndex: 0, PhysicalNPUs: 8},
			},
			Slices: []model.SliceUnit{
				{ID: "slice-train-01-0", CardID: "card-node-train-01-0", FlavorID: "ascend-910b2-whole-card", Available: true},
			},
		},
	}
}

func (l *Ledger) Catalog() *catalog.Root {
	return l.cat
}

func (l *Ledger) ListNodes() []model.Node {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]model.Node, len(l.nodes))
	copy(out, l.nodes)
	return out
}

func (l *Ledger) GetNode(name string) (model.Node, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, n := range l.nodes {
		if n.Name == name {
			return n, true
		}
	}
	return model.Node{}, false
}

func (l *Ledger) Summary() []model.LedgerSummary {
	l.mu.RLock()
	defer l.mu.RUnlock()
	type key struct{ pool, flavor string }
	counts := make(map[key]struct{ cap, alloc int })
	for _, n := range l.nodes {
		for _, s := range n.Slices {
			k := key{pool: n.PoolID, flavor: s.FlavorID}
			c := counts[k]
			c.cap++
			if !s.Available {
				c.alloc++
			}
			counts[k] = c
		}
	}
	out := make([]model.LedgerSummary, 0, len(counts))
	for k, v := range counts {
		out = append(out, model.LedgerSummary{
			PoolID:         k.pool,
			FlavorID:       k.flavor,
			CapacityUnits:  v.cap,
			AllocatedUnits: v.alloc,
			FreeUnits:      v.cap - v.alloc,
		})
	}
	return out
}

type CreateAllocationInput struct {
	Tenant    string
	PoolID    string
	FlavorID  string
	Namespace string
	PodRef    string
}

func (l *Ledger) CreateAllocation(in CreateAllocationInput) (*model.Allocation, error) {
	if _, ok := l.cat.FlavorByID(in.FlavorID); !ok {
		return nil, ErrUnknownFlavor
	}
	pool, ok := l.cat.PoolByID(in.PoolID)
	if !ok {
		return nil, ErrUnknownPool
	}
	supported := false
	for _, fid := range pool.SupportedFlavorIDs {
		if fid == in.FlavorID {
			supported = true
			break
		}
	}
	if !supported {
		return nil, ErrFlavorNotInPool
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var picked *model.SliceUnit
	for i := range l.nodes {
		if l.nodes[i].PoolID != in.PoolID {
			continue
		}
		for j := range l.nodes[i].Slices {
			s := &l.nodes[i].Slices[j]
			if s.FlavorID == in.FlavorID && s.Available {
				picked = s
				break
			}
		}
		if picked != nil {
			break
		}
	}
	if picked == nil {
		return nil, ErrNoCapacity
	}

	l.nextAllocSeq++
	id := fmt.Sprintf("alloc-%06d", l.nextAllocSeq)
	now := time.Now().UTC()
	a := &model.Allocation{
		ID:          id,
		Tenant:      in.Tenant,
		PoolID:      in.PoolID,
		FlavorID:    in.FlavorID,
		Phase:       model.PhaseBound,
		SliceUnitID: picked.ID,
		Namespace:   in.Namespace,
		PodRef:      in.PodRef,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	picked.Available = false
	l.allocations[id] = a
	return a, nil
}

func (l *Ledger) ListAllocations() []*model.Allocation {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]*model.Allocation, 0, len(l.allocations))
	for _, a := range l.allocations {
		out = append(out, a)
	}
	return out
}

func (l *Ledger) GetAllocation(id string) (*model.Allocation, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	a, ok := l.allocations[id]
	return a, ok
}

func (l *Ledger) ReleaseAllocation(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.allocations[id]
	if !ok {
		return ErrUnknownAlloc
	}
	for i := range l.nodes {
		for j := range l.nodes[i].Slices {
			if l.nodes[i].Slices[j].ID == a.SliceUnitID {
				l.nodes[i].Slices[j].Available = true
				break
			}
		}
	}
	a.Phase = model.PhaseReleased
	a.UpdatedAt = time.Now().UTC()
	return nil
}

func (l *Ledger) K8sBindingForFlavor(flavorID string) (model.K8sResourceBinding, error) {
	f, ok := l.cat.FlavorByID(flavorID)
	if !ok {
		return model.K8sResourceBinding{}, ErrUnknownFlavor
	}
	lim := map[string]string{}
	for k, v := range f.PodLimitsExample {
		lim[k] = v
	}
	return model.K8sResourceBinding{
		FlavorID:            f.ID,
		K8sExtendedResource: f.K8sExtendedResource,
		RecommendedLimits:   lim,
		RecommendedRequests: lim,
		SchedulingHints: map[string]string{
			"device_plugin_note": "vNPU 需先在节点创建后由插件上报；整卡与切片资源键互斥，勿在同一 Pod 混填。",
		},
	}, nil
}
