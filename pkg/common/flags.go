package common

import "github.com/spf13/cobra"

func InitFlags(cloudProvider string, startCmd *cobra.Command) {
	switch cloudProvider {
	case "aws":
		initAWSFlags(startCmd)
	case "scaleway":
		initScalewayFlags(startCmd)
	case "do":
		initDOFlags(startCmd)
	}
}

func initAWSFlags(startCmd *cobra.Command) {
	startCmd.Flags().StringSliceP("aws-regions", "a", nil, "Set AWS regions")
	startCmd.Flags().BoolP("enable-eks", "e", false, "Enable EKS watch")
	startCmd.Flags().BoolP("enable-rds", "r", false, "Enable RDS databases and and its children (subnet groups & parameter groups) watch")
	startCmd.Flags().BoolP("enable-documentdb", "m", false, "Enable DocumentDB watch")
	startCmd.Flags().BoolP("enable-elasticache", "c", false, "Enable Elasticache watch")
	startCmd.Flags().BoolP("enable-elb", "l", false, "Enable Elastic Load Balancers watch (true is eks is enabled)")
	startCmd.Flags().BoolP("enable-ebs", "b", false, "Enable Elastic Volumes watch (true is eks is enabled)")
	startCmd.Flags().BoolP("enable-vpc", "p", false, "Enable VPC and its children (internet gateways, route tables, subnets, security groups) watch")
	startCmd.Flags().BoolP("enable-s3", "s", false, "Enable S3 watch")
	startCmd.Flags().BoolP("enable-cloudwatch-logs", "w", false, "Enable Cloudwatch Logs watch")
	startCmd.Flags().BoolP("enable-kms", "n", false, "Enable KMS watch")
	startCmd.Flags().BoolP("enable-iam", "u", false, "Enable IAM (groups, policies, roles, users) watch")
	startCmd.Flags().BoolP("enable-ssh-keys", "z", false, "Enable Key Pair watch")
	startCmd.Flags().BoolP("enable-ecr", "o", false, "Enable ECR watch")
	startCmd.Flags().BoolP("enable-cloudformation", "d", false, "Enable Cloudformation watch")
}

func initScalewayFlags(startCmd *cobra.Command) {
	startCmd.Flags().StringSliceP("scw-zones", "a", nil, "Set Scaleway regions")
	startCmd.Flags().BoolP("enable-cluster", "e", false, "Enable Kubernetes clusters watch")
	startCmd.Flags().BoolP("enable-db", "r", false, "Enable databases watch")
	startCmd.Flags().BoolP("enable-cr", "o", false, "Enable containers registries watch")
	startCmd.Flags().BoolP("enable-s3", "s", false, "Enable buckets watch")
	startCmd.Flags().BoolP("enable-lb", "l", false, "Enable load balancers watch")
	startCmd.Flags().BoolP("enable-volume", "b", false, "Enable volumes watch")
	startCmd.Flags().BoolP("enable-sg", "p", false, "Enable security groups watch")

}

func initDOFlags(startCmd *cobra.Command) {
	startCmd.Flags().StringSliceP("do-regions", "a", nil, "Set Digital Ocean regions")
	startCmd.Flags().BoolP("enable-cluster", "e", false, "Enable Kubernetes clusters watch")
	startCmd.Flags().BoolP("enable-db", "r", false, "Enable databases watch")
	startCmd.Flags().BoolP("enable-s3", "s", false, "Enable buckets watch")
	startCmd.Flags().BoolP("enable-lb", "l", false, "Enable load balancers watch")
	startCmd.Flags().BoolP("enable-volume", "b", false, "Enable volumes watch")
	startCmd.Flags().BoolP("enable-firewall", "f", false, "Enable firewalls watch")
	startCmd.Flags().BoolP("enable-vpc", "v", false, "Enable VPCs watch")
}
