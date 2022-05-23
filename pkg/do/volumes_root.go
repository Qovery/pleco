package do

import (
	"context"
	"fmt"
	"github.com/digitalocean/godo"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/Qovery/pleco/pkg/common"
)

type DOVolume struct {
	ID           string
	Name         string
	CreationDate time.Time
}

func DeleteExpiredVolumes(sessions DOSessions, options DOOptions) {
	expiredVolumes := getDetachedVolumes(sessions.Client, options.Region)

	count, start := common.ElemToDeleteFormattedInfos(fmt.Sprintf("detached (%d hours delay) volume", volumeTimeout()), len(expiredVolumes), options.Region)

	log.Debug(count)

	if options.DryRun || len(expiredVolumes) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredVolume := range expiredVolumes {
		deleteVolume(sessions.Client, expiredVolume)
	}
}

func getVolumes(client *godo.Client, region string) []DOVolume {
	result, _, err := client.Storage.ListVolumes(context.TODO(), &godo.ListVolumeParams{Region: region})
	if err != nil {
		log.Errorf("Can't list volumes in zone %s: %s", region, err.Error())
		return []DOVolume{}
	}

	volumes := []DOVolume{}
	for _, volume := range result {
		creationDate, _ := time.Parse(time.RFC3339, volume.CreatedAt.Format(time.RFC3339))
		v := DOVolume{
			ID:           volume.ID,
			Name:         volume.Name,
			CreationDate: creationDate,
		}

		volumes = append(volumes, v)
	}

	return volumes
}

func getDetachedVolumes(client *godo.Client, region string) []DOVolume {
	volumes := getVolumes(client, region)

	detachedVolumes := []DOVolume{}
	for _, volume := range volumes {
		// do we need to force delete every volume on detroy command ?
		if volume.CreationDate.UTC().Add(volumeTimeout() * time.Hour).Before(time.Now().UTC()) {
			detachedVolumes = append(detachedVolumes, volume)
		}
	}

	return detachedVolumes
}

func deleteVolume(client *godo.Client, detachedVolume DOVolume) {
	_, err := client.Storage.DeleteVolume(context.TODO(), detachedVolume.ID)

	if err != nil {
		log.Errorf("Can't delete detached volume %s: %s", detachedVolume.Name, err.Error())
	}
}
