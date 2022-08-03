package scaleway

import (
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type ScalewaySecurityGroup struct {
	ID          string
	Name        string
	UpdateDate  time.Time
	IsDefault   bool
	IsAttached  bool
	TTL         int64
	IsProtected bool
}

func DeleteDetachedSecurityGroups(sessions ScalewaySessions, options ScalewayOptions) {
	detachedSGs, _ := getDetachedSG(sessions.SG, &options)

	count, start := common.ElemToDeleteFormattedInfos("detached security group", len(detachedSGs), options.Zone, true)

	log.Info(count)

	if options.DryRun || len(detachedSGs) == 0 {
		return
	}

	log.Info(start)

	for _, detachedSG := range detachedSGs {
		deleteSG(sessions.SG, detachedSG, options.Zone)
	}
}

func listSecurityGroups(instanceAPI *instance.API) ([]ScalewaySecurityGroup, string) {
	input := &instance.ListSecurityGroupsRequest{}
	result, err := instanceAPI.ListSecurityGroups(input)
	region := GetRegionfromZone(input.Zone.String())

	if err != nil {
		log.Errorf("Can't list cluster for region %s: %s", region, err.Error())
		return []ScalewaySecurityGroup{}, region
	}

	SGs := []ScalewaySecurityGroup{}
	for _, sg := range result.SecurityGroups {
		updateDate, _ := time.Parse(time.RFC3339, sg.ModificationDate.Format(time.RFC3339))

		SGs = append(SGs, ScalewaySecurityGroup{
			ID:          sg.ID,
			Name:        sg.Name,
			UpdateDate:  updateDate,
			IsDefault:   sg.ProjectDefault && sg.OrganizationDefault, //nolint:staticcheck
			IsAttached:  len(sg.Servers) > 0,
			TTL:         0,
			IsProtected: false,
		})
	}

	return SGs, region
}

func getDetachedSG(instanceAPI *instance.API, options *ScalewayOptions) ([]ScalewaySecurityGroup, string) {
	SGs, region := listSecurityGroups(instanceAPI)

	detachedSgs := []ScalewaySecurityGroup{}
	for _, SG := range SGs {
		if !SG.IsDefault && !SG.IsAttached &&
			(options.IsDestroyingCommand || SG.UpdateDate.UTC().Add(6*time.Hour).Before(time.Now().UTC())) {
			detachedSgs = append(detachedSgs, SG)
		}
	}

	return detachedSgs, region
}

func deleteSG(instanceAPI *instance.API, securityGroup ScalewaySecurityGroup, region string) {
	err := instanceAPI.DeleteSecurityGroup(
		&instance.DeleteSecurityGroupRequest{
			SecurityGroupID: securityGroup.ID,
		},
	)

	if err != nil {
		log.Errorf("Can't delete security group %s: %s", securityGroup.Name, err.Error())
	} else {
		log.Debugf("Database %s in %s deleted.", securityGroup.Name, region)
	}
}
