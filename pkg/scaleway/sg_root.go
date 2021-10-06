package scaleway

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	log "github.com/sirupsen/logrus"
	"time"
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

func DeleteDetachedSecurityGroups(sessions *ScalewaySessions, options *ScalewayOptions) {
	detachedSGs, region := getDetachedSG(sessions.SG)

	count, start := common.ElemToDeleteFormattedInfos("detached security group", len(detachedSGs), region)

	log.Debug(count)

	if options.DryRun || len(detachedSGs) == 0 {
		return
	}

	log.Debug(start)

	for _, detachedSG := range detachedSGs {
		deleteSG(sessions.SG, detachedSG)
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
			IsDefault:   sg.ProjectDefault && sg.OrganizationDefault,
			IsAttached:  len(sg.Servers) > 0,
			TTL:         0,
			IsProtected: false,
		})
	}

	return SGs, region
}

func getDetachedSG(instanceAPI *instance.API) ([]ScalewaySecurityGroup, string) {
	SGs, region := listSecurityGroups(instanceAPI)

	detachedSgs := []ScalewaySecurityGroup{}
	for _, SG := range SGs {
		if SG.UpdateDate.UTC().Add(6*time.Hour).Before(time.Now().UTC()) && !SG.IsDefault && !SG.IsAttached {
			detachedSgs = append(detachedSgs, SG)
		}
	}

	return detachedSgs, region
}

func deleteSG(instanceAPI *instance.API, securityGroup ScalewaySecurityGroup) {
	err := instanceAPI.DeleteSecurityGroup(
		&instance.DeleteSecurityGroupRequest{
			SecurityGroupID: securityGroup.ID,
		},
	)

	if err != nil {
		log.Errorf("Can't delete security group %s: %s", securityGroup.Name, err.Error())
	}
}
