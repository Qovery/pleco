package do

import (
	"github.com/digitalocean/godo"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type DOOptions struct {
	TagName             string
	TagValue            string
	IsDestroyingCommand bool
	DryRun              bool
	Region              string
	EnableCluster       bool
	EnableDB            bool
	EnableBucket        bool
	EnableLB            bool
	EnableVolume        bool
	EnableFirewall      bool
	EnableVPC           bool
}

type DOSessions struct {
	Client *godo.Client
	Bucket *minio.Client
}

type funcDeleteExpired func(sessions DOSessions, options DOOptions)

func RunPlecoDO(regions []string, interval int64, wg *sync.WaitGroup, options DOOptions) {
	for _, region := range regions {
		wg.Add(1)
		go runPlecoInRegion(region, interval, wg, options)
	}

	wg.Add(1)
	go runPleco(interval, wg, options)
}

func runPlecoInRegion(region string, interval int64, wg *sync.WaitGroup, options DOOptions) {
	defer wg.Done()
	sessions := DOSessions{}
	sessions.Client = CreateSession()
	options.Region = region

	logrus.Infof("Starting to check expired resources in region %s.", options.Region)

	var listServiceToCheckStatus []funcDeleteExpired

	if options.EnableCluster {
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredClusters)
	}

	if options.EnableDB {
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredDatabases)
	}

	if options.EnableVolume {
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredVolumes)
	}

	if options.EnableBucket {
		sessions.Bucket = CreateMinIOSession(options.Region)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredBuckets)
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

func runPleco(interval int64, wg *sync.WaitGroup, options DOOptions) {
	defer wg.Done()
	sessions := DOSessions{}
	sessions.Client = CreateSession()

	logrus.Info("Starting to check global expired resources.")

	var listServiceToCheckStatus []funcDeleteExpired

	if options.EnableLB {
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredLBs)
	}

	if options.EnableFirewall {
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredFirewalls)
	}

	if options.EnableVPC {
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredVPCs)
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
