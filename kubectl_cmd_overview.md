# EKS Cluster Resource Analysis — kubectl Command Guide

---

## 1. Node-Level Overview

### Actual Usage

```bash
kubectl top nodes
```

Shows real-time CPU and memory **consumption** per node. Requires metrics-server.

### Allocated (Requested) Resources

```bash
kubectl describe nodes | grep -A 8 "Allocated resources"
```

Shows CPU and memory **requests and limits** summed across all pods on each node.

### How to Compare

Place outputs side by side. The gap between "Allocated requests %" and "Actual usage %" reveals over-provisioning. For
example:

| Node   | CPU Requested | CPU Actual | Verdict                            |
|--------|---------------|------------|------------------------------------|
| node-A | 76%           | 3%         | Massively over-requested           |
| node-B | 9%            | 2%         | Under-utilized and under-requested |

If requested is high but actual is low → pods are **reserving far more than they use**.
If scheduling fails despite low actual usage → requests have consumed all allocatable capacity.

---

## 2. Pod-Level Resource Requests & Limits

### All Pods on a Specific Node

```bash
kubectl get pods --all-namespaces -o json | jq -r '
  .items[] | select(.spec.nodeName=="<NODE_NAME>") |
  .metadata.namespace + "\t" + .metadata.name + "\t" +
  ([.spec.containers[].resources.requests.cpu // "0"] | join(",")) + "\t" +
  ([.spec.containers[].resources.limits.cpu // "0"] | join(",")) + "\t" +
  ([.spec.containers[].resources.requests.memory // "0"] | join(",")) + "\t" +
  ([.spec.containers[].resources.limits.memory // "0"] | join(","))
'
```

Replace `<NODE_NAME>` with the full node name (e.g., `ip-10-1-4-238.eu-central-1.compute.internal`).

Shows what each pod on that node has **requested and limited** for CPU and memory. Multi-container pods show
comma-separated values.

### Actual Usage per Pod

```bash
kubectl top pods --all-namespaces --no-headers | sort -k3 -rn | head -30
```

Shows real-time CPU and memory usage. Sorting by column 3 (CPU) reveals the heaviest consumers.

### How to Compare

Cross-reference requests (from jq command) with actual usage (from `kubectl top`). Calculate the over-request factor:

```
Over-request factor = CPU Request / CPU Actual
```

A factor of 10x+ means the pod is reserving 10 times what it needs. Typical findings in test environments show factors
of 50-500x for idle Java/Kotlin services.

---

## 3. Cluster-Wide Top Requesters

### Top 25 Pods by CPU Requests

```bash
kubectl get pods --all-namespaces -o json | jq -r '
  [.items[] |
   {ns: .metadata.namespace, name: .metadata.name,
    cpu_req: ([.spec.containers[].resources.requests.cpu // "0" |
               if endswith("m") then rtrimstr("m") | tonumber
               else tonumber * 1000 end] | add)}] |
  sort_by(-.cpu_req) | .[:25][] |
  "\(.cpu_req)m\t\(.ns)\t\(.name)"
'
```

Identifies which pods are consuming the most **scheduling capacity** cluster-wide. These are your primary targets for
right-sizing.

### Compare With

Run `kubectl top pods --all-namespaces` and compare. Pods that appear high in requests but absent from the top actual
usage list are the biggest waste.

---

## 4. Specific Pod Inspection

### Check Requests/Limits for Individual Pods

```bash
kubectl get pods --all-namespaces -o json | jq -r '
  .items[] | select(
    .metadata.name == "<POD_NAME_1>" or
    .metadata.name == "<POD_NAME_2>"
  ) |
  .metadata.namespace + "\t" + .metadata.name + "\t" +
  ([.spec.containers[].resources.requests.cpu // "0"] | join(",")) + "\t" +
  ([.spec.containers[].resources.requests.memory // "0"] | join(",")) + "\t" +
  ([.spec.containers[].resources.limits.memory // "0"] | join(","))
'
```

Use this to investigate outliers — pods with zero requests (no guaranteed resources), pods exceeding their requests, or
pods with suspiciously high reservations.

---

## 5. DaemonSets (Per-Node Overhead)

```bash
kubectl get daemonsets --all-namespaces -o wide
```

DaemonSets run on **every node**. Their resource requests multiply by node count. Common culprits: Datadog agent,
aws-node (VPC CNI), kube-proxy, ebs-csi-node, ingress-nginx (if DaemonSet), Fluent Bit.

### Check Their Requests

```bash
kubectl get ds --all-namespaces -o json | jq -r '
  .items[] |
  .metadata.namespace + "\t" + .metadata.name + "\t" +
  ([.spec.template.spec.containers[].resources.requests.cpu // "0"] | join(",")) + "\t" +
  ([.spec.template.spec.containers[].resources.requests.memory // "0"] | join(","))
'
```

---

## 6. Node Constraints (Taints & Labels)

### Check Taints

```bash
kubectl get nodes -o custom-columns="NAME:.metadata.name","TAINTS:.spec.taints"
```

Taints prevent pods from scheduling on nodes unless the pod has a matching toleration. If nodes appear free but pods
can't schedule, taints are a likely cause.

### Check Labels

```bash
kubectl get nodes --show-labels
```

Pods with `nodeSelector` or `nodeAffinity` rules can only schedule on nodes with matching labels, even if other nodes
have capacity.

