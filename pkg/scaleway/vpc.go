package scaleway

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/scaleway/scaleway-sdk-go/api/vpc/v2"
	log "github.com/sirupsen/logrus"
	"time"
)

type ScalewayVPC struct {
	Name string
	common.CloudProviderResource
}

func DeleteExpiredVPCs(sessions ScalewaySessions, options ScalewayOptions) {
	expiredVPCs, _ := getExpiredVPCs(sessions.Network, options)

	count, start := common.ElemToDeleteFormattedInfos("expired VPC", len(expiredVPCs), options.Zone, true)

	log.Info(count)

	if options.DryRun || len(expiredVPCs) == 0 {
		return
	}

	log.Info(start)

	for _, expiredVPC := range expiredVPCs {
		log.Info("Deleting VPC: ", expiredVPC.Name)
		if err := sessions.Network.DeleteVPC(&vpc.DeleteVPCRequest{
			Region: options.Region,
			VpcID:  expiredVPC.Identifier,
		}); err != nil {
			log.Errorf("Error deleting VPC %s: %v", expiredVPC.Name, err)
		}
	}
}

func getExpiredVPCs(vpcAPI *vpc.API, options ScalewayOptions) ([]ScalewayVPC, error) {
	// List all IPs
	expiredVPCs := make([]ScalewayVPC, 0)
	var page int32 = 1
	var itemsPerPage uint32 = 100

	for {
		vpcsResponse, err := vpcAPI.ListVPCs(&vpc.ListVPCsRequest{
			Page:     &page,
			PageSize: &itemsPerPage,
			Region:   options.Region,
		})
		if err != nil {
			log.Fatalf("Error listing VPCs: %v", err)
			return nil, err
		}

		for _, vpcItem := range vpcsResponse.Vpcs {
			essentialTags := common.GetEssentialTags(vpcItem.Tags, options.TagName)
			creationDate, _ := time.Parse(time.RFC3339, vpcItem.CreatedAt.Format(time.RFC3339))

			vpcResource := ScalewayVPC{
				CloudProviderResource: common.CloudProviderResource{
					Identifier:   vpcItem.ID,
					Description:  "VPC: " + vpcItem.Name,
					CreationDate: creationDate,
					TTL:          essentialTags.TTL,
					Tag:          essentialTags.Tag,
					IsProtected:  essentialTags.IsProtected,
				},
				Name: vpcItem.Name,
			}

			if vpcResource.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
				expiredVPCs = append(expiredVPCs, vpcResource)
			}
		}

		if vpcsResponse.TotalCount <= uint32(page)*itemsPerPage {
			break
		}

		page += 1
	}

	return expiredVPCs, nil
}
