package scaleway

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	log "github.com/sirupsen/logrus"
	"time"
)

type ScalewayVolume struct {
	ID        string
	Name      string
	UpdatedAt time.Time
	State     string
}

func DeleteExpiredVolumes(sessions *ScalewaySessions, options *ScalewayOption) {
	expiredVolumes, zone := getDetachedVolumes(sessions.Volume)

	count, start := common.ElemToDeleteFormattedInfos("detached volume", len(expiredVolumes), zone)

	log.Debug(count)

	if options.DryRun || len(expiredVolumes) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredVolume := range expiredVolumes {
		deleteVolume(sessions.Volume, expiredVolume)
	}
}

func getVolumes(volumeAPI *instance.API) ([]ScalewayVolume, string) {
	input := &instance.ListVolumesRequest{
	}
	zone := input.Zone.String()
	result, err := volumeAPI.ListVolumes(input)
	if err != nil {
		log.Errorf("Can't list volumes in zone %s: %s", zone, err.Error())
	}

	volumes := []ScalewayVolume{}
	for _, volume := range result.Volumes {
		creationDate, _ := time.Parse(time.RFC3339, volume.ModificationDate.Format(time.RFC3339))
		volumes = append(volumes, ScalewayVolume{
			ID:        volume.ID,
			Name:      volume.Name,
			UpdatedAt: creationDate,
			State:     volume.State.String(),
		})
	}

	return volumes, zone
}

func getDetachedVolumes(volumeAPI *instance.API) ([]ScalewayVolume, string) {
	volumes, zone := getVolumes(volumeAPI)

	detachedVolumes := []ScalewayVolume{}
	for _, volume := range volumes {
		if volume.UpdatedAt.Add(2*time.Hour).After(time.Now()) && volume.State == "" {
			detachedVolumes = append(detachedVolumes, volume)
		}
	}

	return detachedVolumes, zone
}

func deleteVolume(volumeAPI *instance.API, detachedVolume ScalewayVolume) {
	err := volumeAPI.DeleteVolume(
		&instance.DeleteVolumeRequest{
			VolumeID: detachedVolume.ID,
		},
	)

	if err != nil {
		log.Errorf("Can't delete detached volume %s: %s", detachedVolume.Name, err.Error())
	}
}
