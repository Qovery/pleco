package scaleway

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/scaleway/scaleway-sdk-go/api/vpc/v2"
	log "github.com/sirupsen/logrus"
	"time"
)

type ScalewayPrivateNetwork struct {
	Name string
	common.CloudProviderResource
}

func DeleteExpiredPrivateNetworks(sessions ScalewaySessions, options ScalewayOptions) {
	expiredPrivateNetworks, _ := getExpiredPrivateNetworks(sessions.Network, options)

	count, start := common.ElemToDeleteFormattedInfos("expired private network", len(expiredPrivateNetworks), options.Zone, true)

	log.Info(count)

	if options.DryRun || len(expiredPrivateNetworks) == 0 {
		return
	}

	log.Info(start)

	for _, expiredPrivateNetwork := range expiredPrivateNetworks {
		log.Info("Deleting private network: ", expiredPrivateNetwork.Name)
		if err := sessions.Network.DeletePrivateNetwork(&vpc.DeletePrivateNetworkRequest{
			Region:           options.Region,
			PrivateNetworkID: expiredPrivateNetwork.Identifier,
		}); err != nil {
			log.Errorf("Error deleting private network %s: %v", expiredPrivateNetwork.Name, err)
		}
	}
}

func getExpiredPrivateNetworks(vpcAPI *vpc.API, options ScalewayOptions) ([]ScalewayPrivateNetwork, error) {
	// List all IPs
	expiredPrivateNetworks := make([]ScalewayPrivateNetwork, 0)
	var page int32 = 1
	var itemsPerPage uint32 = 100

	for {
		privateNetworksResponse, err := vpcAPI.ListPrivateNetworks(&vpc.ListPrivateNetworksRequest{
			Page:     &page,
			PageSize: &itemsPerPage,
			Region:   options.Region,
		})
		if err != nil {
			log.Fatalf("Error listing private networks: %v", err)
			return nil, err
		}

		for _, privateNetwork := range privateNetworksResponse.PrivateNetworks {
			essentialTags := common.GetEssentialTags(privateNetwork.Tags, options.TagName)
			creationDate, _ := time.Parse(time.RFC3339, privateNetwork.CreatedAt.Format(time.RFC3339))

			vpcResource := ScalewayPrivateNetwork{
				CloudProviderResource: common.CloudProviderResource{
					Identifier:   privateNetwork.ID,
					Description:  "PrivateNetwork: " + privateNetwork.Name,
					CreationDate: creationDate,
					TTL:          essentialTags.TTL,
					Tag:          essentialTags.Tag,
					IsProtected:  essentialTags.IsProtected,
				},
				Name: privateNetwork.Name,
			}

			if vpcResource.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
				expiredPrivateNetworks = append(expiredPrivateNetworks, vpcResource)
			}
		}

		if privateNetworksResponse.TotalCount <= uint32(page)*itemsPerPage {
			break
		}

		page += 1
	}

	return expiredPrivateNetworks, nil
}
