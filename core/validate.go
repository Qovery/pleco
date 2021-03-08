package core

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

func isAwsUsed(cmd *cobra.Command, serviceName string) bool {
	service, err := cmd.Flags().GetBool("enable-" + serviceName)
	if err == nil && service {
		return true
	}
	return false
}

func checkEnvVars(cmd *cobra.Command) {
	var requiredEnvVars []string
	awsEnvVars := []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
	}

	// if kubernetes is required
	kubeConn, err := cmd.Flags().GetString("kube-conn")
	if err == nil && kubeConn == "out" {
		requiredEnvVars = append(requiredEnvVars, "KUBECONFIG")
	}

	// if an AWS service is required
	if isAwsUsed(cmd, "rds") ||
		isAwsUsed(cmd, "documentdb") ||
		isAwsUsed(cmd, "elasticache") ||
		isAwsUsed(cmd, "eks") ||
		isAwsUsed(cmd, "elb") ||
		isAwsUsed(cmd, "vpc") ||
		isAwsUsed(cmd, "s3") ||
		isAwsUsed(cmd, "ebs") ||
		isAwsUsed(cmd, "kms"){
		requiredEnvVars = append(requiredEnvVars, awsEnvVars...)
	}

	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("%s environment variable is required and not found", envVar)
		}
	}
}