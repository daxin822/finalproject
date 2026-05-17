package orchestration

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"
)

// RenderTrainingYAML 根据 MindCluster + Volcano 约定渲染训练 Job 清单（分布式时前置 Headless master Service）。
func RenderTrainingYAML(in TrainingWorkloadSpec) (string, error) {
	n := normalizeTraining(in)
	if err := validateTrainingDistributed(n); err != nil {
		return "", err
	}
	if err := maybeWrapRankCommand(&n); err != nil {
		return "", err
	}
	var docs []string

	if n.Distributed.Enabled && distributedCreateMasterService(n.Distributed) {
		svc, err := renderMasterServiceYAML(n)
		if err != nil {
			return "", err
		}
		docs = append(docs, strings.TrimSpace(svc))
	}

	job, err := renderTrainJobYAML(n)
	if err != nil {
		return "", err
	}
	docs = append(docs, strings.TrimSpace(job))
	if len(docs) == 1 {
		return docs[0] + "\n", nil
	}
	return strings.Join(docs, "\n---\n") + "\n", nil
}

func renderTrainJobYAML(n TrainingWorkloadSpec) (string, error) {
	t, err := template.New("train_volcano.yaml.tmpl").Funcs(template.FuncMap{
		"quote": yamlQuote,
	}).ParseFS(templateFS, "templates/train_volcano.yaml.tmpl")
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, n); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderMasterServiceYAML(n TrainingWorkloadSpec) (string, error) {
	t, err := template.New("train_master_svc.yaml.tmpl").Funcs(template.FuncMap{
		"quote": yamlQuote,
	}).ParseFS(templateFS, "templates/train_master_svc.yaml.tmpl")
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, n); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderInferenceYAML 渲染推理 Deployment（可选 Service）。
func RenderInferenceYAML(in InferenceWorkloadSpec) (string, error) {
	n := normalizeInference(in)
	t, err := template.New("inference-deployment.yaml.tmpl").Funcs(template.FuncMap{
		"quote": yamlQuote,
	}).ParseFS(templateFS, "templates/inference-deployment.yaml.tmpl")
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, n); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()) + "\n", nil
}

func yamlQuote(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, `"'#\n:\t{}[],&*!|>%`) && s[0] != ' ' && s[len(s)-1] != ' ' {
		return s
	}
	return fmt.Sprintf("%q", s)
}

func normalizeTraining(in TrainingWorkloadSpec) TrainingWorkloadSpec {
	out := in
	out.Scheduler = defaultScheduler(out.Scheduler)
	out.Replicas = defaultPositive(out.Replicas, 1)
	out.MinAvailable = defaultPositive(out.MinAvailable, 1)
	if out.MinAvailable > out.Replicas {
		out.MinAvailable = out.Replicas
	}
	if out.TaskName == "" {
		out.TaskName = "training-worker"
	}
	if out.ResQuantity == "" {
		out.ResQuantity = "1"
	}
	if out.Distributed.Enabled {
		if out.Distributed.WorldSize <= 0 {
			out.Distributed.WorldSize = out.Replicas
		}
		if out.Distributed.MasterPort <= 0 {
			out.Distributed.MasterPort = 29500
		}
		if out.Distributed.HCCLTimeout <= 0 {
			out.Distributed.HCCLTimeout = 7200
		}
		if out.Distributed.DeviceBackend == "" {
			out.Distributed.DeviceBackend = "hccl"
		}
		if out.Distributed.MasterAddr == "" && distributedCreateMasterService(out.Distributed) {
			out.Distributed.MasterAddr = fmt.Sprintf("%s-master.%s.svc.cluster.local",
				out.Name, out.Namespace)
		}
	}
	if out.Labels == nil {
		out.Labels = map[string]string{}
	}
	out.Labels["control-plane.finalproject/workload-name"] = out.Name
	if out.Annotations == nil {
		out.Annotations = map[string]string{}
	}
	if out.NodeSelector == nil {
		out.NodeSelector = map[string]string{}
	}
	return out
}

func distributedCreateMasterService(d DistributedParams) bool {
	if d.CreateMasterService != nil {
		return *d.CreateMasterService
	}
	return true
}

func validateTrainingDistributed(n TrainingWorkloadSpec) error {
	if n.Replicas > 1 && n.Distributed.Enabled && !n.Distributed.RankFromPodName {
		return fmt.Errorf("distributed training with replicas>1 requires distributed.rank_from_pod_name")
	}
	return nil
}

// maybeWrapRankCommand 多副本时通过 bash 从 POD_NAME 后缀解析 RANK（Volcano Pod 名通常以 -0,-1 结尾）。
func maybeWrapRankCommand(out *TrainingWorkloadSpec) error {
	if !out.Distributed.Enabled || out.Replicas <= 1 {
		return nil
	}
	userLine := shellExecLine(out.Command, out.Args)
	script := "set -euo pipefail\n" +
		`export RANK="${POD_NAME##*-}"` + "\n" +
		"export LOCAL_RANK=0\n" +
		userLine + "\n"
	out.Command = []string{"/bin/bash", "-c", script}
	out.Args = nil
	return nil
}

func shellExecLine(cmd, args []string) string {
	parts := append(append([]string{}, cmd...), args...)
	if len(parts) == 0 {
		return "sleep 60"
	}
	var b strings.Builder
	b.WriteString("exec")
	for _, p := range parts {
		b.WriteByte(' ')
		b.WriteString(strconv.Quote(p))
	}
	return b.String()
}

func normalizeInference(in InferenceWorkloadSpec) InferenceWorkloadSpec {
	out := in
	out.Replicas = defaultPositive(out.Replicas, 1)
	if out.Port <= 0 {
		out.Port = 8080
	}
	if out.ResQuantity == "" {
		out.ResQuantity = "1"
	}
	out.ServiceType = defaultString(out.ServiceType, "ClusterIP")
	if out.Labels == nil {
		out.Labels = map[string]string{}
	}
	if out.Annotations == nil {
		out.Annotations = map[string]string{}
	}
	if out.NodeSelector == nil {
		out.NodeSelector = map[string]string{}
	}
	if out.ResourceRequests == nil {
		out.ResourceRequests = map[string]string{}
	}
	return out
}
