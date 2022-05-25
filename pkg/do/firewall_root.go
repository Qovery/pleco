package do

import (
	"context"
	"fmt"
	"github.com/digitalocean/godo"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/Qovery/pleco/pkg/common"
)

type DOFirewall struct {
	ID           string
	Name         string
	CreationDate time.Time
	Droplets     []int
}

func DeleteExpiredFirewalls(sessions DOSessions, options DOOptions) {
	expiredFirewalls := getDetachedFirewalls(sessions.Client, &options)

	count, start := common.ElemToDeleteFormattedInfos(fmt.Sprintf("detached (%d hours delay) firewall", volumeTimeout()), len(expiredFirewalls), options.Region)

	log.Debug(count)

	if options.DryRun || len(expiredFirewalls) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredFirewall := range expiredFirewalls {
		deleteFirewall(sessions.Client, expiredFirewall)
	}
}

func getFirewalls(client *godo.Client) []DOFirewall {
	result, _, err := client.Firewalls.List(context.TODO(), &godo.ListOptions{
		PerPage: 200,
	})
	if err != nil {
		log.Errorf("Can't list firewalls: %s", err.Error())
		return []DOFirewall{}
	}

	firewalls := []DOFirewall{}
	for _, firewall := range result {

		creationDate, _ := time.Parse(time.RFC3339, firewall.Created)
		fw := DOFirewall{
			ID:           firewall.ID,
			Name:         firewall.Name,
			CreationDate: creationDate.UTC(),
			Droplets:     firewall.DropletIDs,
		}

		firewalls = append(firewalls, fw)
	}

	return firewalls
}

func getDetachedFirewalls(client *godo.Client, options *DOOptions) []DOFirewall {
	firewalls := getFirewalls(client)

	detachedFirewalls := []DOFirewall{}
	for _, firewall := range firewalls {
		if len(firewall.Droplets) == 0 &&
			(options.isDestroyingCommand() || firewall.CreationDate.UTC().Add(volumeTimeout()*time.Hour).Before(time.Now().UTC())) {
			detachedFirewalls = append(detachedFirewalls, firewall)
		}
	}

	return detachedFirewalls
}

func deleteFirewall(client *godo.Client, detachedFirewall DOFirewall) {
	_, err := client.Firewalls.Delete(context.TODO(), detachedFirewall.ID)

	if err != nil {
		log.Errorf("Can't delete detached firewall %s: %s", detachedFirewall.Name, err.Error())
	}
}
