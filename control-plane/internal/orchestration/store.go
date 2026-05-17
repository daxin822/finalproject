package orchestration

import (
	"sync"
	"time"

	"finalproject/control-plane/internal/model"
)

// WorkloadStore 进程内任务状态库。
type WorkloadStore struct {
	mu      sync.RWMutex
	byID    map[string]*model.WorkloadRecord
	watchers map[string]map[chan *model.WorkloadRecord]struct{}
}

func NewWorkloadStore() *WorkloadStore {
	return &WorkloadStore{
		byID:     make(map[string]*model.WorkloadRecord),
		watchers: make(map[string]map[chan *model.WorkloadRecord]struct{}),
	}
}

func (s *WorkloadStore) Create(rec *model.WorkloadRecord) {
	if rec == nil {
		return
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := copyRecord(rec)
	s.byID[rec.ID] = cp
	s.notifyLocked(cp)
}

func (s *WorkloadStore) Get(id string) (*model.WorkloadRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.byID[id]
	if !ok {
		return nil, false
	}
	return copyRecord(rec), true
}

// List 按 tenant / phase 过滤；空字符串表示不过滤。
func (s *WorkloadStore) List(tenant string, phase model.WorkloadPhase) []*model.WorkloadRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.WorkloadRecord, 0, len(s.byID))
	for _, rec := range s.byID {
		if tenant != "" && rec.Tenant != tenant {
			continue
		}
		if phase != "" && rec.Phase != phase {
			continue
		}
		out = append(out, copyRecord(rec))
	}
	return out
}

// Update 用 fn 修改记录；返回是否找到。
func (s *WorkloadStore) Update(id string, fn func(*model.WorkloadRecord)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.byID[id]
	if !ok {
		return false
	}
	fn(rec)
	rec.UpdatedAt = time.Now().UTC()
	cp := copyRecord(rec)
	s.notifyLocked(cp)
	return true
}

func (s *WorkloadStore) UpdatePhase(id string, phase model.WorkloadPhase, message string) bool {
	return s.Update(id, func(rec *model.WorkloadRecord) {
		rec.Phase = phase
		if message != "" {
			rec.Message = message
		}
	})
}

func (s *WorkloadStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return false
	}
	delete(s.byID, id)
	if subs, ok := s.watchers[id]; ok {
		for ch := range subs {
			close(ch)
		}
		delete(s.watchers, id)
	}
	return true
}

// Subscribe 订阅某任务 phase 变更；调用方应在结束后 Unsubscribe。
func (s *WorkloadStore) Subscribe(id string) chan *model.WorkloadRecord {
	ch := make(chan *model.WorkloadRecord, 8)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.watchers[id] == nil {
		s.watchers[id] = make(map[chan *model.WorkloadRecord]struct{})
	}
	s.watchers[id][ch] = struct{}{}
	return ch
}

func (s *WorkloadStore) Unsubscribe(id string, ch chan *model.WorkloadRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if subs, ok := s.watchers[id]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(s.watchers, id)
		}
	}
}

func (s *WorkloadStore) notifyLocked(rec *model.WorkloadRecord) {
	subs := s.watchers[rec.ID]
	for ch := range subs {
		select {
		case ch <- copyRecord(rec):
		default:
		}
	}
}

func copyRecord(rec *model.WorkloadRecord) *model.WorkloadRecord {
	if rec == nil {
		return nil
	}
	cp := *rec
	if rec.K8sRefs != nil {
		cp.K8sRefs = append([]string(nil), rec.K8sRefs...)
	}
	if rec.PodNames != nil {
		cp.PodNames = append([]string(nil), rec.PodNames...)
	}
	if rec.EventsSummary != nil {
		cp.EventsSummary = append([]string(nil), rec.EventsSummary...)
	}
	return &cp
}
