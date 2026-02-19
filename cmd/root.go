package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"kusa/internal/kube"
)

var (
	kubeconfig  string
	kubeContext string
	clients     *kube.Clients
)

var rootCmd = &cobra.Command{
	Use:   "kusa",
	Short: "Kubernetes Usage Analyzer",
	Long: `kusa surfaces the gap between actual resource usage and requested/allocated
resources in your Kubernetes cluster. This gap is the root cause of
"no resources available" errors on under-utilized clusters: pods reserve
far more than they need, blocking scheduling for others.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		clients, err = kube.NewClients(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("failed to connect to cluster: %w", err)
		}
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file (default: ~/.kube/config)")
	rootCmd.PersistentFlags().StringVar(&kubeContext, "context", "", "Kubernetes context to use (default: current context)")
}
