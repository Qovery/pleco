package azure

import (
	armresources "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/sirupsen/logrus"

	"sync"
	"time"
)

type AzureOptions struct {
	TagValue             string
	TagName              string
	DisableTTLCheck      bool
	IsDestroyingCommand  bool
	DryRun               bool
	Location            string
	SubscriptionID      string
	ResourceGroupName   string
	EnableRG        	 bool
}

type AzureSessions struct {
	RG *armresources.ResourceGroupsClient
}

type funcDeleteExpired func(sessions AzureSessions, options AzureOptions)

func RunPlecoAzure(locations []string, interval int64, wg *sync.WaitGroup, options AzureOptions) {
	for _, location := range locations {
		wg.Add(1)
		go runPlecoInRegion(location, interval, wg, options)
	}
}

func runPlecoInRegion(location string, interval int64, wg *sync.WaitGroup, options AzureOptions) {
	defer wg.Done()
	options.Location = location
	sessions := AzureSessions{}

	logrus.Infof("Starting to check expired resources in location %s.", options.Location)

	var listServiceToCheckStatus []funcDeleteExpired

	if options.EnableRG {
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredRGs)
	}

	if options.IsDestroyingCommand {
		for _, check := range listServiceToCheckStatus {
			check(sessions, options)
		}
	} else {
		for {
			for _, check := range listServiceToCheckStatus {
				check(sessions, options)
			}
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
}
