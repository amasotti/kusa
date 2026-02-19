package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"kusa/internal/kube"
	"kusa/internal/output"
)

var (
	podsLimit         int
	podsIncludeSystem bool
)

var podsCmd = &cobra.Command{
	Use:   "pods",
	Short: "List top pods by CPU request with actual usage",
	Long: `Lists the top N pods cluster-wide by CPU request, cross-referenced with
actual usage from metrics-server. Highlights pods with the highest
over-request factor (CPU requested / CPU actual).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := kube.FetchPods(context.Background(), clients)
		if err != nil {
			return err
		}
		output.RenderPods(result, clients.ContextName, podsIncludeSystem, podsLimit)
		return nil
	},
}

func init() {
	podsCmd.Flags().IntVarP(&podsLimit, "limit", "n", 25, "number of top pods to show")
	podsCmd.Flags().BoolVar(&podsIncludeSystem, "include-system", false, "include system namespaces (kube-system etc.)")
	rootCmd.AddCommand(podsCmd)
}
