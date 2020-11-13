package pleco

import (
	"fmt"
	"github.com/Qovery/pleco/providers/aws"
	log "github.com/sirupsen/logrus"
	"time"
)

func StartDaemon(dryRun bool) {
	fmt.Println("\n ____  _     _____ ____ ___  \n|  _ \\| |   | ____/ ___/ _ \\ \n| |_) | |   |  _|| |  | | | |\n|  __/| |___| |__| |__| |_| |\n|_|   |_____|_____\\____\\___/\nBy Qovery\n")
	log.Info("Starting Pleco")
	if dryRun {
		log.Info("Dry run mode enabled")
	}

	region := "us-east-2"

	// AWS session
	currentSession, err := aws.CreateSession(region)
	if err != nil {
		log.Errorf("AWS session error: %s", err)
	}

	// check RDS
	currentRdsSession := aws.RdsSession(*currentSession, region)

	for {
		err = aws.DeleteExpiredDatabases(*currentRdsSession, "ttl", dryRun)
		if err != nil {
			log.Error(err)
		}
		// check DocumentDB
		err = aws.DeleteExpiredClusters(*currentRdsSession, "ttl", dryRun)
		if err != nil {
			log.Error(err)
		}

		time.Sleep(10 * time.Second)
	}
}