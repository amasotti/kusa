package kube

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// WorkloadInfo holds aggregated resource data for a single workload controller.
type WorkloadInfo struct {
	Kind      string // Deployment, StatefulSet, DaemonSet, Job, ReplicaSet, Pod
	Namespace string
	Name      string
	PodCount  int

	CPURequest int64   // millicores — sum across all pods
	CPUActual  int64   // millicores
	MemRequest float64 // MiB
	MemActual  float64 // MiB

	MetricsAvailable bool
}

// FetchWorkloadsResult holds the result of FetchWorkloads.
type FetchWorkloadsResult struct {
	Workloads        []WorkloadInfo
	MetricsAvailable bool
}

// ownerKey identifies a workload controller.
type ownerKey struct {
	Kind      string
	Namespace string
	Name      string
}

// FetchWorkloads fetches pods, pod metrics, and ReplicaSets concurrently, then
// aggregates pod resource data grouped by the owning workload controller.
func FetchWorkloads(ctx context.Context, clients *Clients, includeSystem bool) (*FetchWorkloadsResult, error) {
	var (
		pods         *corev1.PodList
		podMetrics   *metricsv1beta1.PodMetricsList
		replicaSets  *appsv1.ReplicaSetList
		metricsAvail = true
	)

	g, gctx := errgroup.WithContext(ctx)

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
		podMetrics, err = clients.Metrics.MetricsV1beta1().PodMetricses("").List(gctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Warning: failed to get pod metrics (metrics-server may not be installed): %v\n", err)
			metricsAvail = false
		}
		return nil
	})

	g.Go(func() error {
		var err error
		replicaSets, err = clients.Core.AppsV1().ReplicaSets("").List(gctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list replicasets: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Build map: "namespace/replicaset-name" → Deployment ownerKey
	rsToDeployment := make(map[string]ownerKey)
	for _, rs := range replicaSets.Items {
		for _, ref := range rs.OwnerReferences {
			if ref.Kind == "Deployment" {
				key := rs.Namespace + "/" + rs.Name
				rsToDeployment[key] = ownerKey{Kind: "Deployment", Namespace: rs.Namespace, Name: ref.Name}
				break
			}
		}
	}

	// Build pod metrics map: "namespace/pod-name" → PodMetrics
	podMetricsMap := make(map[string]metricsv1beta1.PodMetrics)
	if podMetrics != nil {
		for _, m := range podMetrics.Items {
			podMetricsMap[m.Namespace+"/"+m.Name] = m
		}
	}

	// Aggregate running pods into workloads
	workloadMap := make(map[string]*WorkloadInfo)

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if !includeSystem && SystemNamespaces[pod.Namespace] {
			continue
		}

		owner := resolveWorkloadOwner(pod, rsToDeployment)
		key := owner.Namespace + "/" + owner.Kind + "/" + owner.Name

		if _, ok := workloadMap[key]; !ok {
			workloadMap[key] = &WorkloadInfo{
				Kind:             owner.Kind,
				Namespace:        owner.Namespace,
				Name:             owner.Name,
				MetricsAvailable: metricsAvail,
			}
		}

		w := workloadMap[key]
		w.PodCount++

		for _, c := range pod.Spec.Containers {
			if q := c.Resources.Requests[corev1.ResourceCPU]; !q.IsZero() {
				w.CPURequest += MillicoresFromQuantity(q)
			}
			if q := c.Resources.Requests[corev1.ResourceMemory]; !q.IsZero() {
				w.MemRequest += MiBFromQuantity(q)
			}
		}

		if metricsAvail {
			pmKey := pod.Namespace + "/" + pod.Name
			if pm, ok := podMetricsMap[pmKey]; ok {
				for _, c := range pm.Containers {
					w.CPUActual += MillicoresFromQuantity(c.Usage[corev1.ResourceCPU])
					w.MemActual += MiBFromQuantity(c.Usage[corev1.ResourceMemory])
				}
			}
		}
	}

	result := &FetchWorkloadsResult{MetricsAvailable: metricsAvail}
	for _, w := range workloadMap {
		result.Workloads = append(result.Workloads, *w)
	}
	return result, nil
}

// resolveWorkloadOwner walks a pod's ownerReferences to find its top-level controller.
// Pod → ReplicaSet → Deployment is resolved via rsToDeployment.
func resolveWorkloadOwner(pod corev1.Pod, rsToDeployment map[string]ownerKey) ownerKey {
	for _, ref := range pod.OwnerReferences {
		switch ref.Kind {
		case "ReplicaSet":
			rsKey := pod.Namespace + "/" + ref.Name
			if dep, ok := rsToDeployment[rsKey]; ok {
				return dep // Pod belongs to a Deployment via its ReplicaSet
			}
			return ownerKey{Kind: "ReplicaSet", Namespace: pod.Namespace, Name: ref.Name}
		case "StatefulSet", "DaemonSet", "Job":
			return ownerKey{Kind: ref.Kind, Namespace: pod.Namespace, Name: ref.Name}
		}
	}
	// Standalone pod — use the pod itself as the "workload"
	return ownerKey{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name}
}
