package pleco

import (
	"fmt"
	"github.com/Qovery/pleco/providers/aws"
	log "github.com/sirupsen/logrus"
)

func StartDaemon() {
	log.Info("Starting Pleco")
	region := "us-east-2"
	currentSession, err := aws.CreateSession(region)
	if err != nil {
		log.Errorf("AWS session error: %s", err)
	}
	databasesTagged, err := aws.ListTaggedDatabases(*currentSession, region, "ttl")
	if err != nil {
		log.Errorf("wasn't able to retrieve database info: %s", err)
	}
	fmt.Println(databasesTagged)
}