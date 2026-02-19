package output

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/amasotti/kusa/internal/analysis"
	"github.com/amasotti/kusa/internal/kube"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

var noColor bool

// SetNoColor disables ANSI color codes in console output.
func SetNoColor(v bool) { noColor = v }

// cellValue holds a text value and optional ANSI colors for console rendering.
type cellValue struct {
	text   string
	colors text.Colors
}

func cv(s string) cellValue                       { return cellValue{text: s} }
func cvColored(s string, c text.Colors) cellValue { return cellValue{text: s, colors: c} }

// renderTable renders a table to stdout (with colors) and returns a markdown string.
func renderTable(title string, headers []string, rows [][]cellValue) string {
	headerRow := make(table.Row, len(headers))
	for i, h := range headers {
		headerRow[i] = h
	}

	// Console table
	console := table.NewWriter()
	console.SetOutputMirror(os.Stdout)
	console.SetTitle(title)
	console.AppendHeader(headerRow)
	for _, row := range rows {
		r := make(table.Row, len(row))
		for i, cell := range row {
			if !noColor && len(cell.colors) > 0 {
				r[i] = cell.colors.Sprint(cell.text)
			} else {
				r[i] = cell.text
			}
		}
		console.AppendRow(r)
	}
	console.SetStyle(table.StyleRounded)
	console.Render()

	// Markdown table (plain text)
	md := table.NewWriter()
	md.AppendHeader(headerRow)
	for _, row := range rows {
		r := make(table.Row, len(row))
		for i, cell := range row {
			r[i] = cell.text
		}
		md.AppendRow(r)
	}
	return md.RenderMarkdown()
}

func safePctInt(value, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(value) * 100 / float64(total)
}

func safePctFloat(value, total float64) float64 {
	if total == 0 {
		return 0
	}
	return value * 100 / total
}

func naCell() cellValue {
	return cvColored("N/A", text.Colors{text.Faint})
}

// meetsFactorFilter reports whether a req/actual pair satisfies a --min-factor threshold.
//
//	threshold == 0 → always true (filter disabled)
//	threshold  > 0 → req/actual >= threshold  (over-requested by at least that factor)
//	threshold  < 0 → actual > req             (bursting; any negative value)
//
// Returns false when req is 0 or metrics are unavailable.
func meetsFactorFilter(req, actual int64, metricsAvail bool, threshold int) bool {
	if threshold == 0 {
		return true
	}
	if req == 0 || !metricsAvail {
		return false
	}
	if threshold < 0 {
		return actual > req
	}
	if actual == 0 {
		return true // requesting but consuming nothing → infinite factor
	}
	return req/actual >= int64(threshold)
}

// verdictFromRatio computes a verdict by treating req as 100% and expressing actual as
// a percentage of it. This makes ResourceVerdict reusable for pods and workloads where
// there is no node-level allocatable capacity to normalise against.
func verdictFromRatio(req, actual float64, metricsAvail bool) cellValue {
	if req == 0 {
		return cvColored("no req", text.Colors{text.Faint})
	}
	if !metricsAvail {
		return naCell()
	}
	v := analysis.ResourceVerdict(100, actual/req*100)
	return cvColored(v.Label, text.Colors{v.Color})
}

// RenderNodes renders the nodes table to stdout and saves markdown files.
func RenderNodes(result *kube.FetchNodesResult, contextName string, includeSystem bool, podOverview bool) {
	ts := time.Now()

	fmt.Println()
	mdContent := renderNodesMain(result, contextName)
	saveMarkdownFile("nodes", contextName, ts, mdContent)

	if podOverview {
		fmt.Println()
		mdContent := renderNodesPodOverview(result, contextName, includeSystem)
		saveMarkdownFile("nodes_pod_overview", contextName, ts, mdContent)
	}
}

func renderNodesMain(result *kube.FetchNodesResult, contextName string) string {
	title := fmt.Sprintf("Nodes — %s", contextName)
	headers := []string{
		"Node",
		"CPU Actual", "CPU Requested", "CPU Verdict",
		"Mem Actual", "Mem Requested", "Mem Verdict",
	}

	var rows [][]cellValue
	for _, node := range result.Nodes {
		cpuActualPct := safePctInt(node.ActualCPU, node.AllocatableCPU)
		cpuReqPct := safePctInt(node.RequestedCPU, node.AllocatableCPU)
		memActualPct := safePctFloat(node.ActualMem, node.AllocatableMem)
		memReqPct := safePctFloat(node.RequestedMem, node.AllocatableMem)

		cpuReqStr := fmt.Sprintf("%.0f%% (%s)", cpuReqPct, kube.FormatCPU(node.RequestedCPU))
		memReqStr := fmt.Sprintf("%.0f%% (%s)", memReqPct, kube.FormatMem(node.RequestedMem))

		var cpuActualCell, memActualCell, cpuVerdictCell, memVerdictCell cellValue
		if result.NodeMetricsAvailable && node.MetricsAvailable {
			cpuActualCell = cv(fmt.Sprintf("%.0f%% (%s)", cpuActualPct, kube.FormatCPU(node.ActualCPU)))
			memActualCell = cv(fmt.Sprintf("%.0f%% (%s)", memActualPct, kube.FormatMem(node.ActualMem)))

			cpuV := analysis.ResourceVerdict(cpuReqPct, cpuActualPct)
			memV := analysis.ResourceVerdict(memReqPct, memActualPct)
			cpuVerdictCell = cvColored(cpuV.Label, text.Colors{cpuV.Color})
			memVerdictCell = cvColored(memV.Label, text.Colors{memV.Color})
		} else {
			cpuActualCell = naCell()
			memActualCell = naCell()
			cpuVerdictCell = naCell()
			memVerdictCell = naCell()
		}

		rows = append(rows, []cellValue{
			cv(node.Name),
			cpuActualCell,
			cv(cpuReqStr),
			cpuVerdictCell,
			memActualCell,
			cv(memReqStr),
			memVerdictCell,
		})
	}

	return renderTable(title, headers, rows)
}

func renderNodesPodOverview(result *kube.FetchNodesResult, contextName string, includeSystem bool) string {
	headers := []string{
		"Namespace", "Pod",
		"CPU Req", "CPU Limit", "CPU Actual", "Over-req",
		"Mem Req", "Mem Limit", "Mem Actual",
	}

	var allRows [][]cellValue
	var allMd string

	for _, node := range result.Nodes {
		pods := node.Pods
		if !includeSystem {
			filtered := pods[:0]
			for _, p := range pods {
				if !kube.SystemNamespaces[p.Namespace] {
					filtered = append(filtered, p)
				}
			}
			pods = filtered
		}
		if len(pods) == 0 {
			continue
		}

		// Sort by CPU request descending
		sort.Slice(pods, func(i, j int) bool {
			return pods[i].CPURequest > pods[j].CPURequest
		})

		nodeTitle := fmt.Sprintf("Pod Overview: %s — %s", node.Name, contextName)
		var rows [][]cellValue

		for _, pod := range pods {
			cpuLimitStr := kube.FormatCPU(pod.CPULimit)
			if pod.CPULimit == 0 {
				cpuLimitStr = "-"
			}
			memLimitStr := kube.FormatMem(pod.MemLimit)
			if pod.MemLimit == 0 {
				memLimitStr = "-"
			}

			factorStr := kube.FormatFactor(pod.CPURequest, pod.CPUActual)
			factorColors := analysis.FactorColors(pod.CPURequest, pod.CPUActual)

			var cpuActualCell, memActualCell cellValue
			if result.PodMetricsAvailable && pod.MetricsAvailable {
				cpuActualCell = cv(kube.FormatCPU(pod.CPUActual))
				memActualCell = cv(kube.FormatMem(pod.MemActual))
			} else {
				cpuActualCell = naCell()
				memActualCell = naCell()
			}

			rows = append(rows, []cellValue{
				cv(pod.Namespace),
				cv(pod.Name),
				cv(kube.FormatCPU(pod.CPURequest)),
				cv(cpuLimitStr),
				cpuActualCell,
				cvColored(factorStr, factorColors),
				cv(kube.FormatMem(pod.MemRequest)),
				cv(memLimitStr),
				memActualCell,
			})
		}

		allRows = append(allRows, rows...)

		fmt.Println()
		mdTable := renderTable(nodeTitle, headers, rows)
		allMd += fmt.Sprintf("## %s\n\n%s\n\n", node.Name, mdTable)
		_ = allRows
	}

	return allMd
}

// RenderDeployments renders workloads grouped by controller to stdout and saves a markdown file.
// Results are sorted by CPU over-request factor descending (worst first).
func RenderDeployments(result *kube.FetchWorkloadsResult, contextName string, limit int, minFactor int) {
	ts := time.Now()

	workloads := make([]kube.WorkloadInfo, len(result.Workloads))
	copy(workloads, result.Workloads)

	// Filter by over-request factor
	if minFactor != 0 {
		filtered := workloads[:0]
		for _, w := range workloads {
			if meetsFactorFilter(w.CPURequest, w.CPUActual, result.MetricsAvailable && w.MetricsAvailable, minFactor) {
				filtered = append(filtered, w)
			}
		}
		workloads = filtered
	}

	sort.Slice(workloads, func(i, j int) bool {
		return workloadSortFactor(workloads[i]) > workloadSortFactor(workloads[j])
	})
	if limit > 0 && len(workloads) > limit {
		workloads = workloads[:limit]
	}

	title := fmt.Sprintf("Deployments — %s", contextName)
	headers := []string{"#", "Kind", "Namespace", "Workload", "Pods", "CPU Req", "CPU Actual", "Over-req", "CPU Verdict", "Mem Req", "Mem Actual", "Mem Verdict"}

	var rows [][]cellValue
	for i, w := range workloads {
		factorStr := kube.FormatFactor(w.CPURequest, w.CPUActual)
		factorColors := analysis.FactorColors(w.CPURequest, w.CPUActual)

		metricsAvail := result.MetricsAvailable && w.MetricsAvailable
		var cpuActualCell, memActualCell cellValue
		if metricsAvail {
			cpuActualCell = cv(kube.FormatCPU(w.CPUActual))
			memActualCell = cv(kube.FormatMem(w.MemActual))
		} else {
			cpuActualCell = naCell()
			memActualCell = naCell()
		}

		rows = append(rows, []cellValue{
			cv(fmt.Sprintf("%d", i+1)),
			cv(w.Kind),
			cv(w.Namespace),
			cv(w.Name),
			cv(fmt.Sprintf("%d", w.PodCount)),
			cv(kube.FormatCPU(w.CPURequest)),
			cpuActualCell,
			cvColored(factorStr, factorColors),
			verdictFromRatio(float64(w.CPURequest), float64(w.CPUActual), metricsAvail),
			cv(kube.FormatMem(w.MemRequest)),
			memActualCell,
			verdictFromRatio(w.MemRequest, w.MemActual, metricsAvail),
		})
	}

	fmt.Println()
	mdContent := renderTable(title, headers, rows)
	saveMarkdownFile("deployments", contextName, ts, mdContent)
}

// workloadSortFactor returns a float64 key for sorting workloads by CPU over-request severity.
// Higher = worse. Unknowns and no-request workloads sort to the bottom.
func workloadSortFactor(w kube.WorkloadInfo) float64 {
	if w.CPURequest == 0 {
		return -1 // no requests set → least interesting
	}
	if !w.MetricsAvailable {
		return -0.5 // can't compare without metrics
	}
	if w.CPUActual == 0 {
		return 1e15 // requesting but consuming nothing → worst case
	}
	return float64(w.CPURequest) / float64(w.CPUActual)
}

// RenderPods renders the pods table to stdout and saves a markdown file.
func RenderPods(result *kube.FetchPodsResult, contextName string, includeSystem bool, limit int, minFactor int) {
	ts := time.Now()

	// Filter system namespaces
	pods := result.Pods
	if !includeSystem {
		filtered := pods[:0]
		for _, p := range pods {
			if !kube.SystemNamespaces[p.Namespace] {
				filtered = append(filtered, p)
			}
		}
		pods = filtered
	}

	// Filter by over-request factor
	if minFactor != 0 {
		filtered := pods[:0]
		for _, p := range pods {
			if meetsFactorFilter(p.CPURequest, p.CPUActual, result.MetricsAvailable && p.MetricsAvailable, minFactor) {
				filtered = append(filtered, p)
			}
		}
		pods = filtered
	}

	// Sort by CPU request descending
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].CPURequest > pods[j].CPURequest
	})

	// Take top N
	if limit > 0 && len(pods) > limit {
		pods = pods[:limit]
	}

	title := fmt.Sprintf("Top Pods — %s", contextName)
	headers := []string{"#", "Namespace", "Pod", "Node", "CPU Req", "CPU Actual", "Over-req", "CPU Verdict", "Mem Req", "Mem Actual", "Mem Verdict"}

	var rows [][]cellValue
	for i, pod := range pods {
		factorStr := kube.FormatFactor(pod.CPURequest, pod.CPUActual)
		factorColors := analysis.FactorColors(pod.CPURequest, pod.CPUActual)

		metricsAvail := result.MetricsAvailable && pod.MetricsAvailable
		var cpuActualCell, memActualCell cellValue
		if metricsAvail {
			cpuActualCell = cv(kube.FormatCPU(pod.CPUActual))
			memActualCell = cv(kube.FormatMem(pod.MemActual))
		} else {
			cpuActualCell = naCell()
			memActualCell = naCell()
		}

		rows = append(rows, []cellValue{
			cv(fmt.Sprintf("%d", i+1)),
			cv(pod.Namespace),
			cv(pod.Name),
			cv(pod.NodeName),
			cv(kube.FormatCPU(pod.CPURequest)),
			cpuActualCell,
			cvColored(factorStr, factorColors),
			verdictFromRatio(float64(pod.CPURequest), float64(pod.CPUActual), metricsAvail),
			cv(kube.FormatMem(pod.MemRequest)),
			memActualCell,
			verdictFromRatio(pod.MemRequest, pod.MemActual, metricsAvail),
		})
	}

	fmt.Println()
	mdContent := renderTable(title, headers, rows)
	saveMarkdownFile("pods", contextName, ts, mdContent)
}
