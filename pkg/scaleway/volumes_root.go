package scaleway

import (
	"fmt"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/Qovery/pleco/pkg/common"
)

type ScalewayVolume struct {
	ID        string
	Name      string
	UpdatedAt time.Time
	ServerId  string
}

func DeleteExpiredVolumes(sessions ScalewaySessions, options ScalewayOptions) {
	expiredVolumes := getDetachedVolumes(sessions.Volume, options.Zone)

	count, start := common.ElemToDeleteFormattedInfos(fmt.Sprintf("detached (%d hours delay) volume", volumeTimeout()), len(expiredVolumes), options.Zone, true)

	log.Debug(count)

	if options.DryRun || len(expiredVolumes) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredVolume := range expiredVolumes {
		deleteVolume(sessions.Volume, expiredVolume)
	}
}

func getVolumes(volumeAPI *instance.API, zone string) []ScalewayVolume {
	input := &instance.ListVolumesRequest{}
	result, err := volumeAPI.ListVolumes(input)
	if err != nil {
		log.Errorf("Can't list volumes in zone %s: %s", zone, err.Error())
		return []ScalewayVolume{}
	}

	volumes := []ScalewayVolume{}
	for _, volume := range result.Volumes {
		updateDate, _ := time.Parse(time.RFC3339, volume.ModificationDate.Format(time.RFC3339))
		v := ScalewayVolume{
			ID:        volume.ID,
			Name:      volume.Name,
			UpdatedAt: updateDate,
			ServerId:  "null",
		}

		if volume.Server != nil {
			v.ServerId = volume.Server.ID
		}

		volumes = append(volumes, v)
	}

	return volumes
}

func getDetachedVolumes(volumeAPI *instance.API, zone string) []ScalewayVolume {
	volumes := getVolumes(volumeAPI, zone)

	detachedVolumes := []ScalewayVolume{}
	for _, volume := range volumes {
		// do we need to force delete every volume on destroy command ?
		if volume.UpdatedAt.UTC().Add(volumeTimeout()*time.Hour).Before(time.Now().UTC()) && volume.ServerId == "null" {
			detachedVolumes = append(detachedVolumes, volume)
		}
	}

	return detachedVolumes
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
