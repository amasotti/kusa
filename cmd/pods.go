package cmd

import (
	"context"

	"github.com/amasotti/kusa/internal/kube"
	"github.com/amasotti/kusa/internal/output"
	"github.com/spf13/cobra"
)

var (
	podsLimit         int
	podsIncludeSystem bool
	podsNamespace     string
	podsMinFactor     int
)

var podsCmd = &cobra.Command{
	Use:   "pods",
	Short: "List top pods by CPU request with actual usage",
	Long: `Lists the top N pods cluster-wide by CPU request, cross-referenced with
actual usage from metrics-server. Highlights pods with the highest
over-request factor (CPU requested / CPU actual).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := kube.FetchPods(context.Background(), clients, podsNamespace)
		if err != nil {
			return err
		}
		// When scoped to a specific namespace, honour its pods regardless of system status.
		includeSystem := podsIncludeSystem || podsNamespace != ""
		output.RenderPods(result, clients.ContextName, includeSystem, podsLimit, podsMinFactor)
		return nil
	},
}

func init() {
	podsCmd.Flags().IntVarP(&podsLimit, "limit", "n", 25, "number of top pods to show")
	podsCmd.Flags().BoolVar(&podsIncludeSystem, "include-system", false, "include system namespaces (kube-system etc.)")
	podsCmd.Flags().StringVar(&podsNamespace, "namespace", "", "filter by namespace (default: all namespaces)")
	podsCmd.Flags().IntVar(&podsMinFactor, "min-factor", 0, "only show pods where CPU req/actual >= N; negative N shows bursting pods (actual > req); 0 disables filter")
	rootCmd.AddCommand(podsCmd)
}
