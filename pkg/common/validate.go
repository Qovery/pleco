package common

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

func isUsed(cmd *cobra.Command, serviceName string) bool {
	service, err := cmd.Flags().GetBool("enable-" + serviceName)
	if err == nil && service {
		return true
	}
	return false
}

func CheckEnvVars(cloudProvider string, cmd *cobra.Command) {
	var requiredEnvVars []string

	switch cloudProvider {
	case "aws":
		requiredEnvVars = append(requiredEnvVars, checkAWSEnvVars(cmd)...)
	case "scaleway":
		requiredEnvVars = append(requiredEnvVars, checkScalewayEnvVars(cmd)...)
	default:
		log.Fatalf("Unknown cloud provider: %s. Should be \"aws\" or \"scaleway\"", cloudProvider)
	}

	kubeConn, err := cmd.Flags().GetString("kube-conn")
	if err == nil && kubeConn == "out" {
		requiredEnvVars = append(requiredEnvVars, "KUBECONFIG")
	}

	for _, envVar := range requiredEnvVars {
		if _, ok := os.LookupEnv(envVar); !ok {
			log.Fatalf("%s environment variable is required and not found", envVar)
		}
	}
}

func checkAWSEnvVars(cmd *cobra.Command) []string {
	var requiredEnvVars = []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
	}

	if isUsed(cmd, "rds") ||
		isUsed(cmd, "documentdb") ||
		isUsed(cmd, "elasticache") ||
		isUsed(cmd, "eks") ||
		isUsed(cmd, "elb") ||
		isUsed(cmd, "vpc") ||
		isUsed(cmd, "s3") ||
		isUsed(cmd, "ebs") ||
		isUsed(cmd, "cloudwatch-logs") ||
		isUsed(cmd, "kms") ||
		isUsed(cmd, "iam") ||
		isUsed(cmd, "ssh-keys") ||
		isUsed(cmd, "ecr") {
		return requiredEnvVars
	}

	return []string{}
}

func checkScalewayEnvVars(cmd *cobra.Command) []string {
	var requiredEnvVars = []string{
		"SCALEWAY_ORGANISATION_ID",
		"SCALEWAY_ACCESS_KEY",
		"SCALEWAY_SECRET_KEY",
	}

	if isUsed(cmd, "cluster") ||
		isUsed(cmd, "db") ||
		isUsed(cmd, "s3") ||
		isUsed(cmd, "cr") ||
		isUsed(cmd, "lb") ||
		isUsed(cmd, "bucket") {
		return requiredEnvVars
	}

	return []string{}
}
