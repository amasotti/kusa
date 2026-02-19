package cmd

import (
	"context"

	"github.com/amasotti/kusa/internal/kube"
	"github.com/amasotti/kusa/internal/output"
	"github.com/spf13/cobra"
)

var (
	deploymentsLimit         int
	deploymentsIncludeSystem bool
)

var deploymentsCmd = &cobra.Command{
	Use:   "deployments",
	Short: "List workloads ranked by CPU over-request factor",
	Long: `Groups running pods by their owning controller (Deployment, StatefulSet,
DaemonSet) and aggregates CPU/memory request vs actual usage per workload.
Results are sorted by CPU over-request factor descending, so the biggest
capacity offenders appear first.

Pods owned by a ReplicaSet are resolved up to their parent Deployment.
Standalone pods (no owner) are listed individually under kind "Pod".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := kube.FetchWorkloads(context.Background(), clients, deploymentsIncludeSystem)
		if err != nil {
			return err
		}
		output.RenderDeployments(result, clients.ContextName, deploymentsLimit)
		return nil
	},
}

func init() {
	deploymentsCmd.Flags().IntVarP(&deploymentsLimit, "limit", "n", 25, "number of top workloads to show (0 = all)")
	deploymentsCmd.Flags().BoolVar(&deploymentsIncludeSystem, "include-system", false, "include system namespaces (kube-system etc.)")
	rootCmd.AddCommand(deploymentsCmd)
}
