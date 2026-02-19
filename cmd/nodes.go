package cmd

import (
	"context"

	"github.com/amasotti/kusa/internal/kube"
	"github.com/amasotti/kusa/internal/output"
	"github.com/spf13/cobra"
)

var (
	nodesPodOverview   bool
	nodesIncludeSystem bool
)

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Compare actual vs requested resources per node",
	Long: `Compares actual node CPU/memory usage (from metrics-server) against
allocated (requested) resources. Surfaces nodes where pods are reserving
far more than they consume.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := kube.FetchNodes(context.Background(), clients, nodesPodOverview)
		if err != nil {
			return err
		}
		output.RenderNodes(result, clients.ContextName, nodesIncludeSystem, nodesPodOverview)
		return nil
	},
}

func init() {
	nodesCmd.Flags().BoolVar(&nodesPodOverview, "pod-overview", false, "also output a per-node pod breakdown")
	nodesCmd.Flags().BoolVar(&nodesIncludeSystem, "include-system", false, "include system namespaces (kube-system etc.) in pod overview")
	rootCmd.AddCommand(nodesCmd)
}
