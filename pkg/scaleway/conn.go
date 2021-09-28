package scaleway

import (
	"github.com/scaleway/scaleway-sdk-go/scw"
	"github.com/sirupsen/logrus"
	"os"
	"strconv"
	"time"
)

func CreateSession(zone scw.Zone) *scw.Client {
	region, zoneErr := zone.Region()
	if zoneErr != nil {
		logrus.Fatalf("Unknown zone %s: %s", zone.String(), zoneErr.Error())
	}

	client, err := scw.NewClient(
		scw.WithDefaultZone(zone),
		scw.WithDefaultRegion(region),
		scw.WithDefaultOrganizationID(os.Getenv("SCW_ORGANISATION_ID")),
		scw.WithAuth(os.Getenv("SCW_ACCESS_KEY"), os.Getenv("SCW_SECRET_KEY")),
	)
	if err != nil {
		logrus.Errorf("Can't connect to Scaleway: %s", err)
		os.Exit(1)
	}

	return client
}

func volumeTimeout() time.Duration {
	env, ok := os.LookupEnv("SCW_VOLUME_TIMEOUT")
	if ok {
		timeout, err := strconv.Atoi(env)
		if err != nil {
			logrus.Errorf("Can't parse VOLUME_TIMEOUT variable. Set to default (2 hours)")
			return 2
		}
		return time.Duration(timeout)
	}

	return 2
}
