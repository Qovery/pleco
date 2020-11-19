package core

import (
	"github.com/Qovery/pleco/providers/aws"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

func StartDaemon(dryRun bool, interval int64) {
	log.Info("\n\n ____  _     _____ ____ ___  \n|  _ \\| |   | ____/ ___/ _ \\ \n| |_) | |   |  _|| |  | | | |\n|  __/| |___| |__| |__| |_| |\n|_|   |_____|_____\\____\\___/\nBy Qovery\n\n")
	log.Info("Starting Pleco")

	if dryRun {
		log.Info("Dry run mode enabled")
	}
	checkEnvVars()

	// AWS session
	currentSession, err := aws.CreateSession(os.Getenv("AWS_DEFAULT_REGION"))
	if err != nil {
		log.Errorf("AWS session error: %s", err)
	}

	currentRdsSession := aws.RdsSession(*currentSession, os.Getenv("AWS_DEFAULT_REGION"))
	currentElasticacheSession := aws.ElasticacheSession(*currentSession, os.Getenv("AWS_DEFAULT_REGION"))

	for {
		// check RDS
		err = aws.DeleteExpiredRDSDatabases(*currentRdsSession, "ttl", dryRun)
		if err != nil {
			log.Error(err)
		}
		// check DocumentDB
		err = aws.DeleteExpiredDocumentDBClusters(*currentRdsSession, "ttl", dryRun)
		if err != nil {
			log.Error(err)
		}
		// check Elasticache
		err = aws.DeleteExpiredElasticacheDatabases(*currentElasticacheSession, "ttl", dryRun)
		if err != nil {
			log.Error(err)
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}
}