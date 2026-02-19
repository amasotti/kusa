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

// RenderPods renders the pods table to stdout and saves a markdown file.
func RenderPods(result *kube.FetchPodsResult, contextName string, includeSystem bool, limit int) {
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

	// Sort by CPU request descending
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].CPURequest > pods[j].CPURequest
	})

	// Take top N
	if limit > 0 && len(pods) > limit {
		pods = pods[:limit]
	}

	title := fmt.Sprintf("Top Pods — %s", contextName)
	headers := []string{"#", "Namespace", "Pod", "Node", "CPU Req", "CPU Actual", "Over-req", "Mem Req", "Mem Actual"}

	var rows [][]cellValue
	for i, pod := range pods {
		factorStr := kube.FormatFactor(pod.CPURequest, pod.CPUActual)
		factorColors := analysis.FactorColors(pod.CPURequest, pod.CPUActual)

		var cpuActualCell, memActualCell cellValue
		if result.MetricsAvailable && pod.MetricsAvailable {
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
			cv(kube.FormatMem(pod.MemRequest)),
			memActualCell,
		})
	}

	fmt.Println()
	mdContent := renderTable(title, headers, rows)
	saveMarkdownFile("pods", contextName, ts, mdContent)
}
