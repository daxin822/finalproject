package orchestration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// Cluster 封装可选的 Kubernetes 客户端；未配置 kubeconfig 时为 nil。
type Cluster struct {
	clientset *kubernetes.Clientset
	dynamic   dynamic.Interface
	mapper    *restmapper.DeferredDiscoveryRESTMapper
}

// TryConnect 尝试加载 kubeconfig / in-cluster 配置；失败时返回 (nil, nil) 表示未启用集群模式。
func TryConnect() (*Cluster, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, nil
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes clientset: %w", err)
	}
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes dynamic: %w", err)
	}
	disc := memory.NewMemCacheClient(cs.Discovery())
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(disc)
	return &Cluster{clientset: cs, dynamic: dc, mapper: mapper}, nil
}

func (c *Cluster) Enabled() bool {
	return c != nil && c.dynamic != nil
}

// ApplyYAML 将多文档 YAML 应用到集群（仅 Create；已存在则返回错误）。
func (c *Cluster) ApplyYAML(ctx context.Context, yaml string) ([]string, error) {
	if !c.Enabled() {
		return nil, errors.New("kubernetes client not configured")
	}
	dec := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(yaml), 1<<20)
	var applied []string
	for {
		var raw map[string]interface{}
		err := dec.Decode(&raw)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return applied, fmt.Errorf("decode manifest: %w", err)
		}
		if len(raw) == 0 {
			continue
		}
		obj := unstructured.Unstructured{Object: raw}
		gvk := obj.GroupVersionKind()
		if gvk.Kind == "" || gvk.Version == "" {
			return applied, fmt.Errorf("manifest missing apiVersion/kind")
		}
		m, err := c.mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
		if err != nil {
			return applied, fmt.Errorf("rest mapping %v: %w", gvk, err)
		}
		ns := obj.GetNamespace()
		name := obj.GetName()
		var dr dynamic.ResourceInterface
		if m.Scope.Name() == meta.RESTScopeNameRoot {
			dr = c.dynamic.Resource(m.Resource)
		} else {
			if ns == "" {
				return applied, fmt.Errorf("%s %s needs namespace", gvk.Kind, name)
			}
			dr = c.dynamic.Resource(m.Resource).Namespace(ns)
		}
		_, err = dr.Create(ctx, &obj, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			return applied, fmt.Errorf("%s %s/%s already exists", gvk.Kind, ns, name)
		}
		if err != nil {
			return applied, fmt.Errorf("create %s %s/%s: %w", gvk.Kind, ns, name, err)
		}
		// 五段式引用，避免 Volcano Job 与 batch/v1 Job 的 Kind 同名冲突。
		refNs := ns
		if m.Scope.Name() == meta.RESTScopeNameRoot {
			refNs = "_"
		}
		ref := workloadRefString(gvk.Group, gvk.Version, gvk.Kind, refNs, name)
		applied = append(applied, ref)
	}
	return applied, nil
}

// PodSummary 用于 Watch / 轮询聚合。
type PodSummary struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Phase     string `json:"phase"`
	NodeName  string `json:"node_name,omitempty"`
}

// ListPods 列出命名空间下符合 labelSelector 的 Pod 摘要。
func (c *Cluster) ListPods(ctx context.Context, namespace, labelSelector string) ([]PodSummary, error) {
	if !c.Enabled() {
		return nil, errors.New("kubernetes client not configured")
	}
	list, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	out := make([]PodSummary, 0, len(list.Items))
	for _, p := range list.Items {
		out = append(out, PodSummary{
			Name:      p.Name,
			Namespace: p.Namespace,
			Phase:     string(p.Status.Phase),
			NodeName:  p.Spec.NodeName,
		})
	}
	return out, nil
}

// workloadRefString 编码为 group/version/kind/ns/name（core 组为空串，ns 对集群资源为空）。
func workloadRefString(group, version, kind, namespace, name string) string {
	return strings.Join([]string{group, version, kind, namespace, name}, "/")
}

// ParseWorkloadRef 解码 ApplyYAML / Store 中保存的引用。
func ParseWorkloadRef(ref string) (group, version, kind, namespace, name string, ok bool) {
	parts := strings.Split(ref, "/")
	if len(parts) != 5 {
		return "", "", "", "", "", false
	}
	return parts[0], parts[1], parts[2], parts[3], parts[4], true
}

// DeleteRef 按五段式引用删除对象（与 ApplyYAML 输出一致）。
func (c *Cluster) DeleteRef(ctx context.Context, ref string) error {
	if !c.Enabled() {
		return errors.New("kubernetes client not configured")
	}
	group, version, kind, ns, name, ok := ParseWorkloadRef(ref)
	if !ok || name == "" || version == "" || kind == "" {
		return fmt.Errorf("invalid k8s ref %q", ref)
	}
	gvk := schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
	m, err := c.mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
	if err != nil {
		return fmt.Errorf("rest mapping %v: %w", gvk, err)
	}
	var dr dynamic.ResourceInterface
	if m.Scope.Name() == meta.RESTScopeNameRoot || ns == "_" {
		dr = c.dynamic.Resource(m.Resource)
	} else {
		if ns == "" {
			return fmt.Errorf("%s %s needs namespace in ref", gvk.Kind, name)
		}
		dr = c.dynamic.Resource(m.Resource).Namespace(ns)
	}
	err = dr.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete %s: %w", ref, err)
	}
	return nil
}

// FetchEventsSummary 返回涉及某对象的最近若干条事件摘要（Reason + Message）。
func (c *Cluster) FetchEventsSummary(ctx context.Context, namespace, involvedName string, limit int) ([]string, error) {
	if !c.Enabled() {
		return nil, errors.New("kubernetes client not configured")
	}
	if limit <= 0 {
		limit = 5
	}
	list, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + involvedName,
	})
	if err != nil {
		return nil, err
	}
	evs := list.Items
	sort.Slice(evs, func(i, j int) bool {
		return evs[i].LastTimestamp.After(evs[j].LastTimestamp.Time)
	})
	out := make([]string, 0, limit)
	for _, e := range evs {
		if len(out) >= limit {
			break
		}
		msg := strings.TrimSpace(e.Message)
		if len(msg) > 120 {
			msg = msg[:117] + "..."
		}
		line := strings.TrimSpace(e.Reason)
		if msg != "" {
			if line != "" {
				line += ": "
			}
			line += msg
		}
		if line == "" {
			line = e.Name
		}
		out = append(out, line)
	}
	return out, nil
}

// GetUnstructured 按 GVR 拉取单个对象。
func (c *Cluster) GetUnstructured(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	if !c.Enabled() {
		return nil, errors.New("kubernetes client not configured")
	}
	var dr dynamic.ResourceInterface
	if namespace == "" {
		dr = c.dynamic.Resource(gvr)
	} else {
		dr = c.dynamic.Resource(gvr).Namespace(namespace)
	}
	obj, err := dr.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// DiscoveryGroups 返回 discovery 摘要（便于组会展示 API 是否可达）。
func (c *Cluster) DiscoveryGroups(ctx context.Context) ([]string, error) {
	if !c.Enabled() {
		return nil, errors.New("kubernetes client not configured")
	}
	groups, err := c.clientset.Discovery().ServerGroups()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, g := range groups.Groups {
		names = append(names, g.Name)
	}
	return names, nil
}

// ClientSet 供高级用例使用。
func (c *Cluster) ClientSet() *kubernetes.Clientset {
	if c == nil {
		return nil
	}
	return c.clientset
}

// WatchPods 监听 Pod 变更（用于 SSE / Informer 式回流）。
func (c *Cluster) WatchPods(ctx context.Context, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	if !c.Enabled() {
		return nil, errors.New("kubernetes client not configured")
	}
	return c.clientset.CoreV1().Pods(namespace).Watch(ctx, opts)
}

// PodPhaseCounts 统计全集群 Pod 阶段分布（需 list/watch 权限；失败时返回错误）。
func (c *Cluster) PodPhaseCounts(ctx context.Context) (map[string]int, error) {
	if !c.Enabled() {
		return nil, errors.New("kubernetes client not configured")
	}
	list, err := c.clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make(map[string]int)
	for _, p := range list.Items {
		ph := string(p.Status.Phase)
		if ph == "" {
			ph = "Unknown"
		}
		out[ph]++
	}
	return out, nil
}
