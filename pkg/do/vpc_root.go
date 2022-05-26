package do

import (
	"context"
	"fmt"
	"github.com/digitalocean/godo"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/Qovery/pleco/pkg/common"
)

type DOVpc struct {
	ID           string
	Name         string
	CreationDate time.Time
	Members      []*godo.VPCMember
}

func DeleteExpiredVPCs(sessions DOSessions, options DOOptions) {
	expiredVPCs := getExpiredVPCs(sessions.Client, &options)

	count, start := common.ElemToDeleteFormattedInfos(fmt.Sprintf("expired (%d hours delay) VPC", volumeTimeout()), len(expiredVPCs), options.Region)

	log.Debug(count)

	if options.DryRun || len(expiredVPCs) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredVPC := range expiredVPCs {
		deleteVPC(sessions.Client, expiredVPC)
	}
}

func getVPCs(client *godo.Client, region string) []DOVpc {
	result, _, err := client.VPCs.List(context.TODO(), &godo.ListOptions{
		PerPage: 100,
	})

	if err != nil {
		log.Errorf("Can't list VPCs in region %s: %s", region, err.Error())
		return []DOVpc{}
	}

	VPCs := []DOVpc{}
	for _, VPC := range result {
		membersResult, _, membersErr := client.VPCs.ListMembers(context.TODO(), VPC.ID, &godo.VPCListMembersRequest{}, &godo.ListOptions{
			PerPage: 200,
		})
		if membersErr != nil || membersResult == nil {
			log.Errorf("Can't list members for VPC %s: %s", VPC.Name, err.Error())
			continue
		}

		if !VPC.Default {
			creationDate, _ := time.Parse(time.RFC3339, VPC.CreatedAt.Format(time.RFC3339))
			v := DOVpc{
				ID:           VPC.ID,
				Name:         VPC.Name,
				CreationDate: creationDate.UTC(),
				Members:      membersResult,
			}

			VPCs = append(VPCs, v)
		}
	}

	return VPCs
}

func getExpiredVPCs(client *godo.Client, options *DOOptions) []DOVpc {
	VPCs := getVPCs(client, options.Region)

	expiredVPCs := []DOVpc{}
	for _, VPC := range VPCs {
		if len(VPC.Members) == 0 &&
			(options.IsDestroyingCommand || VPC.CreationDate.UTC().Add(volumeTimeout()*time.Hour).Before(time.Now().UTC())) {
			expiredVPCs = append(expiredVPCs, VPC)
		}
	}

	return expiredVPCs
}

func deleteVPC(client *godo.Client, detachedVPC DOVpc) {
	_, err := client.VPCs.Delete(context.TODO(), detachedVPC.ID)

	if err != nil {
		log.Errorf("Can't delete expired VPC %s: %s", detachedVPC.Name, err.Error())
	}
}
