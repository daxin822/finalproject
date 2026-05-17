package metrics

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
)

var (
	mu sync.Mutex

	httpBy = map[string]int64{} // key method|code

	allocActive int64
	wlByPhase   = map[string]int64{}
	k8sPending  int64
)

func IncHTTP(method, code string) {
	key := method + "|" + code
	mu.Lock()
	httpBy[key]++
	mu.Unlock()
}

func SetAllocationsActive(n int64) { atomic.StoreInt64(&allocActive, n) }

func SetWorkloadPhase(phase string, n int64) {
	mu.Lock()
	wlByPhase[phase] = n
	mu.Unlock()
}

func SetK8sPodsPending(n int64) { atomic.StoreInt64(&k8sPending, n) }

func ResetWorkloadPhases() {
	mu.Lock()
	wlByPhase = map[string]int64{}
	mu.Unlock()
}

// Handler 输出 Prometheus 文本指标（无第三方依赖）。
func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(render()))
}

func render() string {
	mu.Lock()
	hb := make(map[string]int64, len(httpBy))
	for k, v := range httpBy {
		hb[k] = v
	}
	wp := make(map[string]int64, len(wlByPhase))
	for k, v := range wlByPhase {
		wp[k] = v
	}
	mu.Unlock()
	a := atomic.LoadInt64(&allocActive)
	kp := atomic.LoadInt64(&k8sPending)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# HELP cp_http_requests_total HTTP requests handled by control-plane.\n")
	fmt.Fprintf(&buf, "# TYPE cp_http_requests_total counter\n")
	for k, v := range hb {
		var method, code string
		for i := 0; i < len(k); i++ {
			if k[i] == '|' {
				method, code = k[:i], k[i+1:]
				break
			}
		}
		fmt.Fprintf(&buf, "cp_http_requests_total{method=%q,code=%q} %d\n", method, code, v)
	}
	fmt.Fprintf(&buf, "# HELP cp_allocations_active Active (non-released) allocations in ledger.\n")
	fmt.Fprintf(&buf, "# TYPE cp_allocations_active gauge\n")
	fmt.Fprintf(&buf, "cp_allocations_active %d\n", a)
	fmt.Fprintf(&buf, "# HELP cp_workloads_total Workloads by phase.\n")
	fmt.Fprintf(&buf, "# TYPE cp_workloads_total gauge\n")
	for ph, v := range wp {
		fmt.Fprintf(&buf, "cp_workloads_total{phase=%q} %d\n", ph, v)
	}
	fmt.Fprintf(&buf, "# HELP cp_k8s_pods_pending Cluster-wide pods in Pending phase (best-effort).\n")
	fmt.Fprintf(&buf, "# TYPE cp_k8s_pods_pending gauge\n")
	fmt.Fprintf(&buf, "cp_k8s_pods_pending %d\n", kp)
	return buf.String()
}
