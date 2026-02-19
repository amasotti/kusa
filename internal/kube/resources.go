package kube

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// SystemNamespaces lists namespaces excluded by default.
var SystemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
}

// NodeInfo holds per-node resource data.
type NodeInfo struct {
	Name           string
	AllocatableCPU int64   // millicores
	AllocatableMem float64 // MiB

	// From metrics API (zero if metrics-server unavailable)
	ActualCPU        int64
	ActualMem        float64
	MetricsAvailable bool

	// Aggregated from all running pods on this node
	RequestedCPU int64
	RequestedMem float64

	// Per-pod breakdown (populated when withPodMetrics=true)
	Pods []PodInfo
}

// PodInfo holds per-pod resource data.
type PodInfo struct {
	Namespace string
	Name      string
	NodeName  string

	CPURequest int64   // millicores
	CPULimit   int64   // millicores (0 = not set)
	MemRequest float64 // MiB
	MemLimit   float64 // MiB (0 = not set)

	CPUActual        int64
	MemActual        float64
	MetricsAvailable bool
}

// MillicoresFromQuantity converts a CPU Quantity to millicores.
func MillicoresFromQuantity(q resource.Quantity) int64 {
	return q.MilliValue()
}

// MiBFromQuantity converts a memory Quantity to MiB.
func MiBFromQuantity(q resource.Quantity) float64 {
	return float64(q.Value()) / (1024 * 1024)
}

// FormatMem formats a MiB value as "512Mi" or "1.5Gi".
func FormatMem(mib float64) string {
	if mib >= 1024 {
		gib := mib / 1024
		if gib == float64(int64(gib)) {
			return fmt.Sprintf("%dGi", int64(gib))
		}
		return fmt.Sprintf("%.1fGi", gib)
	}
	return fmt.Sprintf("%dMi", int64(mib))
}

// FormatCPU formats millicores as "250m" or "1.5" (cores) when >= 1000m.
func FormatCPU(millicores int64) string {
	if millicores == 0 {
		return "0"
	}
	if millicores < 1000 {
		return fmt.Sprintf("%dm", millicores)
	}
	cores := float64(millicores) / 1000
	if float64(int64(cores)) == cores {
		return fmt.Sprintf("%d", int64(cores))
	}
	return fmt.Sprintf("%.2f", cores)
}

// FormatFactor returns the over-request factor string: "42x", "N/A" (actual=0), or "no req" (req=0).
func FormatFactor(req, actual int64) string {
	if req == 0 {
		return "no req"
	}
	if actual == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%dx", req/actual)
}

// FetchNodesResult holds the result of FetchNodes.
type FetchNodesResult struct {
	Nodes                []NodeInfo
	NodeMetricsAvailable bool
	PodMetricsAvailable  bool
}

// FetchNodes fetches nodes, pods, node metrics, and (optionally) pod metrics concurrently.
func FetchNodes(ctx context.Context, clients *Clients, withPodMetrics bool) (*FetchNodesResult, error) {
	var (
		nodes       *corev1.NodeList
		pods        *corev1.PodList
		nodeMetrics *metricsv1beta1.NodeMetricsList
		podMetrics  *metricsv1beta1.PodMetricsList

		nodeMetricsAvail = true
		podMetricsAvail  = true
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		nodes, err = clients.Core.CoreV1().Nodes().List(gctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list nodes: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		var err error
		pods, err = clients.Core.CoreV1().Pods("").List(gctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		var err error
		nodeMetrics, err = clients.Metrics.MetricsV1beta1().NodeMetricses().List(gctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Warning: failed to get node metrics (metrics-server may not be installed): %v\n", err)
			nodeMetricsAvail = false
		}
		return nil
	})

	if withPodMetrics {
		g.Go(func() error {
			var err error
			podMetrics, err = clients.Metrics.MetricsV1beta1().PodMetricses("").List(gctx, metav1.ListOptions{})
			if err != nil {
				fmt.Printf("Warning: failed to get pod metrics: %v\n", err)
				podMetricsAvail = false
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Build node metrics map
	nodeMetricsMap := make(map[string]metricsv1beta1.NodeMetrics)
	if nodeMetrics != nil {
		for _, m := range nodeMetrics.Items {
			nodeMetricsMap[m.Name] = m
		}
	}

	// Build pod metrics map
	podMetricsMap := make(map[string]metricsv1beta1.PodMetrics)
	if podMetrics != nil {
		for _, m := range podMetrics.Items {
			podMetricsMap[m.Namespace+"/"+m.Name] = m
		}
	}

	// Group running pods by node
	podsByNode := make(map[string][]corev1.Pod)
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if pod.Spec.NodeName != "" {
			podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], pod)
		}
	}

	result := &FetchNodesResult{
		NodeMetricsAvailable: nodeMetricsAvail,
		PodMetricsAvailable:  withPodMetrics && podMetricsAvail,
	}

	for _, node := range nodes.Items {
		ni := NodeInfo{
			Name:           node.Name,
			AllocatableCPU: MillicoresFromQuantity(node.Status.Allocatable[corev1.ResourceCPU]),
			AllocatableMem: MiBFromQuantity(node.Status.Allocatable[corev1.ResourceMemory]),
		}

		if m, ok := nodeMetricsMap[node.Name]; ok {
			ni.ActualCPU = MillicoresFromQuantity(m.Usage[corev1.ResourceCPU])
			ni.ActualMem = MiBFromQuantity(m.Usage[corev1.ResourceMemory])
			ni.MetricsAvailable = true
		}

		for _, pod := range podsByNode[node.Name] {
			pi := podInfoFromPod(pod)

			if withPodMetrics {
				key := pod.Namespace + "/" + pod.Name
				if pm, ok := podMetricsMap[key]; ok {
					pi.MetricsAvailable = true
					for _, c := range pm.Containers {
						pi.CPUActual += MillicoresFromQuantity(c.Usage[corev1.ResourceCPU])
						pi.MemActual += MiBFromQuantity(c.Usage[corev1.ResourceMemory])
					}
				}
			}

			// Always include all pods (including system) in node totals
			ni.RequestedCPU += pi.CPURequest
			ni.RequestedMem += pi.MemRequest
			ni.Pods = append(ni.Pods, pi)
		}

		result.Nodes = append(result.Nodes, ni)
	}

	return result, nil
}

// FetchPodsResult holds the result of FetchPods.
type FetchPodsResult struct {
	Pods             []PodInfo
	MetricsAvailable bool
}

// FetchPods fetches running pods and their metrics concurrently.
// When namespace is non-empty only that namespace is queried; pass "" for cluster-wide.
func FetchPods(ctx context.Context, clients *Clients, namespace string) (*FetchPodsResult, error) {
	var (
		pods         *corev1.PodList
		podMetrics   *metricsv1beta1.PodMetricsList
		metricsAvail = true
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		pods, err = clients.Core.CoreV1().Pods(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		var err error
		podMetrics, err = clients.Metrics.MetricsV1beta1().PodMetricses(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Warning: failed to get pod metrics (metrics-server may not be installed): %v\n", err)
			metricsAvail = false
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	podMetricsMap := make(map[string]metricsv1beta1.PodMetrics)
	if podMetrics != nil {
		for _, m := range podMetrics.Items {
			podMetricsMap[m.Namespace+"/"+m.Name] = m
		}
	}

	result := &FetchPodsResult{MetricsAvailable: metricsAvail}

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		pi := podInfoFromPod(pod)

		key := pod.Namespace + "/" + pod.Name
		if pm, ok := podMetricsMap[key]; ok {
			pi.MetricsAvailable = true
			for _, c := range pm.Containers {
				pi.CPUActual += MillicoresFromQuantity(c.Usage[corev1.ResourceCPU])
				pi.MemActual += MiBFromQuantity(c.Usage[corev1.ResourceMemory])
			}
		}

		result.Pods = append(result.Pods, pi)
	}

	return result, nil
}

func podInfoFromPod(pod corev1.Pod) PodInfo {
	pi := PodInfo{
		Namespace: pod.Namespace,
		Name:      pod.Name,
		NodeName:  pod.Spec.NodeName,
	}
	for _, c := range pod.Spec.Containers {
		if q := c.Resources.Requests[corev1.ResourceCPU]; !q.IsZero() {
			pi.CPURequest += MillicoresFromQuantity(q)
		}
		if q := c.Resources.Limits[corev1.ResourceCPU]; !q.IsZero() {
			pi.CPULimit += MillicoresFromQuantity(q)
		}
		if q := c.Resources.Requests[corev1.ResourceMemory]; !q.IsZero() {
			pi.MemRequest += MiBFromQuantity(q)
		}
		if q := c.Resources.Limits[corev1.ResourceMemory]; !q.IsZero() {
			pi.MemLimit += MiBFromQuantity(q)
		}
	}
	return pi
}
