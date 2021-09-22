package scaleway

import (
	"github.com/scaleway/scaleway-sdk-go/scw"
	"github.com/sirupsen/logrus"
	"os"
)

func CreateSession() *scw.Client {
	client, err := scw.NewClient(
		scw.WithDefaultOrganizationID(os.Getenv("SCALEWAY_ORGANISATION_ID")),
		scw.WithAuth(os.Getenv("SCALEWAY_ACCESS_KEY"), os.Getenv("SCALEWAY_SECRET_KEY")),
	)
	if err != nil {
		logrus.Errorf("Can't connect to Scaleway: %s", err)
		os.Exit(1)
	}

	return client
}
