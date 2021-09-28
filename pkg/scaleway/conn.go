package scaleway

import (
	"github.com/scaleway/scaleway-sdk-go/scw"
	"github.com/sirupsen/logrus"
	"os"
)

func CreateSession(zone scw.Zone) *scw.Client {
	region, zoneErr := zone.Region()
	if zoneErr != nil {
		logrus.Fatalf("Unknown zone %s: %s", zone.String(), zoneErr.Error())
	}

	client, err := scw.NewClient(
		scw.WithDefaultZone(zone),
		scw.WithDefaultRegion(region),
		scw.WithDefaultOrganizationID(os.Getenv("SCALEWAY_ORGANISATION_ID")),
		scw.WithAuth(os.Getenv("SCALEWAY_ACCESS_KEY"), os.Getenv("SCALEWAY_SECRET_KEY")),
	)
	if err != nil {
		logrus.Errorf("Can't connect to Scaleway: %s", err)
		os.Exit(1)
	}

	return client
}
