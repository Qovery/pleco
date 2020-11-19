package core

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

func checkEnvVars(cmd *cobra.Command) {
	requiredEnvVars := []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_DEFAULT_REGION",
	}

	kubeConn, err := cmd.Flags().GetString("kube-conn")
	if err == nil && kubeConn == "out" {
		requiredEnvVars = append(requiredEnvVars, "KUBECONFIG")
	}

	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("%s environment variable is required and not found", envVar)
		}
	}
}