package core

import (
	"github.com/Qovery/pleco/providers/aws"
	"github.com/Qovery/pleco/providers/k8s"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"os"
	"time"
)

func StartDaemon(disableDryRun bool, interval int64, cmd *cobra.Command) {
	dryRun := true
	if disableDryRun {
		dryRun = false
	} else {
		log.Info("Dry run mode enabled")
	}
	checkEnvVars(cmd)

	// AWS session
	currentSession, err := aws.CreateSession(os.Getenv("AWS_DEFAULT_REGION"))
	if err != nil {
		log.Errorf("AWS session error: %s", err)
	}

	// RDS + DocumentDB connection
	var currentRdsSession *rds.RDS
	rdsEnabled, _ := cmd.Flags().GetBool("enable-rds")
	documentdbEnabled, _ := cmd.Flags().GetBool("enable-documentdb")
	if rdsEnabled || documentdbEnabled {
		currentRdsSession = aws.RdsSession(*currentSession, os.Getenv("AWS_DEFAULT_REGION"))
	}

	// Elasticache connection
	var currentElasticacheSession *elasticache.ElastiCache
	elasticacheEnabled, _ := cmd.Flags().GetBool("enable-elasticache")
	if elasticacheEnabled {
		currentElasticacheSession = aws.ElasticacheSession(*currentSession, os.Getenv("AWS_DEFAULT_REGION"))
	}

	// Kubernetes connection
	var k8sClientSet *kubernetes.Clientset
	kubernetesEnabled := true
	KubernetesConn, _ := cmd.Flags().GetString("kube-conn")
	switch KubernetesConn {
	case "in":
		k8sClientSet, err = k8s.AuthenticateInCluster()
	case "out":
		k8sClientSet, err = k8s.AuthenticateOutOfCluster()
	default:
		kubernetesEnabled = false
	}
	if err != nil {
		log.Errorf("failed to authenticate on kubernetes with %s connection: %v", KubernetesConn, err)
	}

	// Todo: use a daemon lib instead of this dirty loop + goroutines
	for {
		// check Kube
		if kubernetesEnabled {
			err = k8s.DeleteExpiredNamespaces(k8sClientSet, "ttl", dryRun)
			if err != nil {
				log.Error(err)
			}
		}

		// check RDS
		if rdsEnabled {
			err = aws.DeleteExpiredRDSDatabases(*currentRdsSession, "ttl", dryRun)
			if err != nil {
				log.Error(err)
			}
		}

		// check DocumentDB
		if documentdbEnabled {
			err = aws.DeleteExpiredDocumentDBClusters(*currentRdsSession, "ttl", dryRun)
			if err != nil {
				log.Error(err)
			}
		}

		// check Elasticache
		if elasticacheEnabled {
			err = aws.DeleteExpiredElasticacheDatabases(*currentElasticacheSession, "ttl", dryRun)
			if err != nil {
				log.Error(err)
			}
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}
}