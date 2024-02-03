package common

import (
	"log"
	"os"

	"github.com/spf13/cobra"
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
	case "do":
		requiredEnvVars = append(requiredEnvVars, checkDOEnvVars(cmd)...)
	case "gcp":
		requiredEnvVars = append(requiredEnvVars, checkGCPEnvVars(cmd)...)
	default:
		log.Fatalf("Unknown cloud provider: %s. Should be \"aws\", \"scaleway\" or \"do\"", cloudProvider)
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
		"SCW_ACCESS_KEY",
		"SCW_SECRET_KEY",
	}

	if isUsed(cmd, "cluster") ||
		isUsed(cmd, "db") ||
		isUsed(cmd, "s3") ||
		isUsed(cmd, "cr") ||
		isUsed(cmd, "lb") ||
		isUsed(cmd, "sg") ||
		isUsed(cmd, "volume") {
		return requiredEnvVars
	}

	return []string{}
}

func checkDOEnvVars(cmd *cobra.Command) []string {
	var requiredEnvVars = []string{
		"DO_API_TOKEN",
		"DO_SPACES_KEY",
		"DO_SPACES_KEY",
	}

	if isUsed(cmd, "cluster") ||
		isUsed(cmd, "db") ||
		isUsed(cmd, "s3") ||
		isUsed(cmd, "lb") ||
		isUsed(cmd, "volume") ||
		isUsed(cmd, "firewall") ||
		isUsed(cmd, "vpc") {
		return requiredEnvVars
	}

	return []string{}
}

func checkGCPEnvVars(cmd *cobra.Command) []string {
	var requiredEnvVars = []string{
		"GOOGLE_APPLICATION_CREDENTIALS_JSON_BASE64",
	}
	if isUsed(cmd, "cluster") ||
		isUsed(cmd, "object-storage") ||
		isUsed(cmd, "artifact-registry") ||
		isUsed(cmd, "network") ||
		isUsed(cmd, "iam") {
		return requiredEnvVars
	}

	return []string{}
}
