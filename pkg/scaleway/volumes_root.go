package scaleway

import (
	"fmt"
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type ScalewayVolume struct {
	ID        string
	Name      string
	UpdatedAt time.Time
	ServerId  string
}

func DeleteExpiredVolumes(sessions ScalewaySessions, options ScalewayOptions) {
	expiredVolumes := getDetachedVolumes(sessions.Volume, &options)

	count, start := common.ElemToDeleteFormattedInfos(fmt.Sprintf("detached (%d hours delay) volume", volumeTimeout()), len(expiredVolumes), options.Zone, true)

	log.Info(count)

	if options.DryRun || len(expiredVolumes) == 0 {
		return
	}

	log.Info(start)

	for _, expiredVolume := range expiredVolumes {
		deleteVolume(sessions.Volume, expiredVolume, options.Zone)
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

func getDetachedVolumes(volumeAPI *instance.API, options *ScalewayOptions) []ScalewayVolume {
	volumes := getVolumes(volumeAPI, options.Zone)

	detachedVolumes := []ScalewayVolume{}
	for _, volume := range volumes {
		if volume.ServerId == "null" &&
			(options.IsDestroyingCommand || volume.UpdatedAt.UTC().Add(volumeTimeout()*time.Hour).Before(time.Now().UTC())) {
			detachedVolumes = append(detachedVolumes, volume)
		}
	}

	return detachedVolumes
}

func deleteVolume(volumeAPI *instance.API, detachedVolume ScalewayVolume, region string) {
	err := volumeAPI.DeleteVolume(
		&instance.DeleteVolumeRequest{
			VolumeID: detachedVolume.ID,
		},
	)

	if err != nil {
		log.Errorf("Can't delete detached volume %s: %s", detachedVolume.Name, err.Error())
	} else {
		log.Debugf("Detached volume %s in %s deleted.", detachedVolume.Name, region)
	}
}
