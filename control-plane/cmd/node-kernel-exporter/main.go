// node-kernel-exporter 提供节点级补充指标（/proc、cgroup v2），用于与 Ascend/MindCluster 设备 exporter 互补。
// 未内嵌 eBPF 字节码：在无 BTF/权限环境下仍可 scrape；生产可替换为 bpftrace/bcc 采集程序并复用同端口格式。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	listen := flag.String("listen", ":9102", "HTTP listen address")
	cgRoot := flag.String("cgroup2-root", "/sys/fs/cgroup", "unified cgroup v2 root (optional)")
	flag.Parse()

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var b strings.Builder
		la := readLoad1m()
		fmt.Fprintf(&b, "# HELP node_load1 1m load average from /proc/loadavg\n# TYPE node_load1 gauge\nnode_load1 %s\n", la)
		if mem := readMemAvailableKB(); mem != "" {
			fmt.Fprintf(&b, "# HELP node_memory_MemAvailable_kilobytes MemAvailable from /proc/meminfo\n# TYPE node_memory_MemAvailable_kilobytes gauge\nnode_memory_MemAvailable_kilobytes %s\n", mem)
		}
		if rt := readTCPRetransSegs(); rt != "" {
			fmt.Fprintf(&b, "# HELP node_netstat_Tcp_RetransSegs Tcp RetransSegs from /proc/net/snmp (累计，适合看增量)\n# TYPE node_netstat_Tcp_RetransSegs counter\nnode_netstat_Tcp_RetransSegs %s\n", rt)
		}
		if cg := cgroupCPUPressure(*cgRoot); cg != "" {
			fmt.Fprintf(&b, "# HELP node_cgroup_cpu_pressure_some cgroup.cpu.pressure some 行（若存在）\n# TYPE node_cgroup_cpu_pressure_some gauge\nnode_cgroup_cpu_pressure_some %s\n", cg)
		}
		fmt.Fprintf(&b, "# HELP node_kernel_exporter_info build info\n# TYPE node_kernel_exporter_info gauge\nnode_kernel_exporter_info{version=\"0.1\"} 1\n")
		_, _ = w.Write([]byte(b.String()))
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })

	s := &http.Server{Addr: *listen, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("node-kernel-exporter listening on %s", *listen)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func readLoad1m() string {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "NaN"
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return "NaN"
	}
	return fields[0]
}

func readMemAvailableKB() string {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemAvailable:") {
			fs := strings.Fields(line)
			if len(fs) >= 2 {
				return fs[1]
			}
		}
	}
	return ""
}

func readTCPRetransSegs() string {
	b, err := os.ReadFile("/proc/net/snmp")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	var tcpHdr, tcpVal []string
	for _, ln := range lines {
		if strings.HasPrefix(ln, "Tcp:") {
			if len(tcpHdr) == 0 {
				tcpHdr = strings.Fields(ln)
			} else if len(tcpVal) == 0 {
				tcpVal = strings.Fields(ln)
				break
			}
		}
	}
	if len(tcpHdr) < 2 || len(tcpVal) != len(tcpHdr) {
		return ""
	}
	idx := -1
	for i, h := range tcpHdr {
		if h == "RetransSegs" {
			idx = i
			break
		}
	}
	if idx < 0 || idx >= len(tcpVal) {
		return ""
	}
	return tcpVal[idx]
}

func cgroupCPUPressure(root string) string {
	p := root + string(os.PathSeparator) + "cpu.pressure"
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	first := strings.TrimSpace(string(b))
	if first == "" {
		return ""
	}
	line := strings.Split(first, "\n")[0]
	for _, f := range strings.Fields(line) {
		if strings.HasPrefix(f, "avg10=") {
			v := strings.TrimPrefix(f, "avg10=")
			if _, err := strconv.ParseFloat(v, 64); err == nil {
				return v
			}
		}
	}
	return ""
}
