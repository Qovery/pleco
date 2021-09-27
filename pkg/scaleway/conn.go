package scaleway

import (
	"github.com/scaleway/scaleway-sdk-go/scw"
	"github.com/sirupsen/logrus"
	"os"
)

func CreateSession(region scw.Region) *scw.Client {
	client, err := scw.NewClient(
		scw.WithDefaultZone(getRegionZone(region)),
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

func getRegionZone(region scw.Region) scw.Zone {
	switch region {
	case scw.RegionFrPar:
		return scw.ZoneFrPar1
	case scw.RegionNlAms:
		return scw.ZoneNlAms1
	case scw.RegionPlWaw:
		return scw.ZonePlWaw1
	default:
		logrus.Errorf("No zone for region %s", region.String())
		return ""
	}
}
