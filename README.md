# kusa — Kubernetes Usage Analyzer

`kusa` surfaces the gap between **actual resource usage** (CPU/memory consumption) and **requested/allocated resources**
(what the Kubernetes scheduler has reserved).

The scheduler allocates capacity based on `requests`, not real usage, so when pods reserve far more than they need,
other pods can't schedule even on nodes that appear to have free capacity.`kusa` makes this waste visible.

At its core is just a tiny cli that uses the golang client-go library to fetch and compare data from the Kubernetes API.
Similar result can be achieved with `kubectl` and `jq` (see [this guide](./kubectl_cmd_overview.md)), but `kusa`
provides a more user-friendly interface, better formatting, and saves results as markdown files for easy sharing.

---

## Installation

**Build from code**
```bash
# Assuming go v1.25+ is installed
git clone
cd kusa
go build -o kusa
```

**Download pre-built binary**
 
see Release page in github.

---

## Prerequisites

- A `kubectl` context configured (`~/.kube/config` or `$KUBECONFIG`)
- [`metrics-server`](https://github.com/kubernetes-sigs/metrics-server) installed in the cluster (required for actual usage data; requests/limits are still shown without it)

---

## Usage

```
kusa [--kubeconfig <path>] [--context <name>] <command> [flags]
```

### Global Flags

| Flag           | Default          | Description               |
|----------------|------------------|---------------------------|
| `--kubeconfig` | `~/.kube/config` | Path to kubeconfig file   |
| `--context`    | current context  | Kubernetes context to use |

---

## Commands

### `kusa nodes`

Compares actual vs requested CPU and memory per node.

```bash
kusa nodes
kusa nodes --pod-overview
kusa nodes --pod-overview --include-system
```

| Flag               | Default | Description                               |
|--------------------|---------|-------------------------------------------|
| `--pod-overview`   | false   | Also show a per-node pod breakdown table  |
| `--include-system` | false   | Include system namespaces in pod overview |

Markdown files are saved to `output/<context>/nodes_<timestamp>.md`.

---

### `kusa pods`

Lists the top N pods by CPU request, cross-referenced with actual usage.

```bash
kusa pods
kusa pods -n 50
kusa pods --include-system
```

| Flag               | Default | Description                |
|--------------------|---------|----------------------------|
| `-n`, `--limit`    | 25      | Number of top pods to show |
| `--include-system` | false   | Include system namespaces  |

Markdown files are saved to `output/<context>/pods_<timestamp>.md`.

---

## How to Interpret Results

**CPU Verdict** and **Mem Verdict** compare requested % vs actual % on each node:

| Condition                                 | Verdict                  |
|-------------------------------------------|--------------------------|
| Requested − Actual > 50 percentage points | Massively over-requested |
| Requested − Actual > 20 percentage points | Over-requested           |
| Actual > Requested                        | Bursting                 |
| Otherwise                                 | OK                       |

**Over-req factor** is `CPU Request / CPU Actual` (integer). A factor of `10x` means a pod requested 10× more CPU than
it actually used. Factors ≥ 10× are highlighted red; ≥ 3× yellow; `N/A` means the pod used 0 CPU (nothing to compare);
`no req` means no CPU request was set.

---

## License

MIT License, see [LICENSE](./LICENSE).