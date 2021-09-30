package scaleway

import (
	"github.com/minio/minio-go/v7"
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/api/rdb/v1"
	"github.com/scaleway/scaleway-sdk-go/api/registry/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type ScalewayOptions struct {
	TagName       string
	DryRun        bool
	Zone          string
	Region scw.Region
	EnableCluster bool
	EnableDB      bool
	EnableCR      bool
	EnableBucket  bool
	EnableLB      bool
	EnableVolume  bool
}

type ScalewaySessions struct {
	Cluster      *k8s.API
	Database     *rdb.API
	Namespace    *registry.API
	LoadBalancer *lb.API
	Volume       *instance.API
	Bucket       *minio.Client
}

type funcDeleteExpired func(sessions *ScalewaySessions, options *ScalewayOptions)

func RunPlecoScaleway(zones []string, interval int64, wg *sync.WaitGroup, options *ScalewayOptions) {
	for _, zone := range zones {
		wg.Add(1)
		go runPlecoInRegion(zone, interval, wg, options)
	}
}

func runPlecoInRegion(zone string, interval int64, wg *sync.WaitGroup, options *ScalewayOptions) {
	defer wg.Done()
	scwZone := scw.Zone(zone)
	sessions := &ScalewaySessions{}
	currentSession := CreateSession(scwZone)
	options.Zone = zone
	options.Region, _ = scwZone.Region()

	logrus.Infof("Starting to check expired resources in zone %s.", zone)

	var listServiceToCheckStatus []funcDeleteExpired

	if options.EnableCluster {
		sessions.Cluster = k8s.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredClusters)
	}

	if options.EnableDB {
		sessions.Database = rdb.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredDatabases)
	}

	if options.EnableCR {
		sessions.Namespace = registry.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteEmptyContainerRegistries)
	}

	if options.EnableLB {
		sessions.LoadBalancer = lb.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredLBs)
	}

	if options.EnableVolume {
		sessions.Volume = instance.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredVolumes)
	}

	if options.EnableBucket {
		sessions.Bucket = CreateMinIOSession(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredBuckets)
	}

	for {
		for _, check := range listServiceToCheckStatus {
			check(sessions, options)
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}

}
