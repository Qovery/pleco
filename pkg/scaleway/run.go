package scaleway

import (
	"github.com/minio/minio-go/v7"
	"sync"
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/api/rdb/v1"
	"github.com/scaleway/scaleway-sdk-go/api/registry/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"github.com/sirupsen/logrus"
)

type ScalewayOptions struct {
	TagValue            string
	TagName             string
	DisableTTLCheck     bool
	IsDestroyingCommand bool
	DryRun              bool
	Zone                string // TODO: use scw.Zone
	Region              scw.Region
	EnableCluster       bool
	EnableDB            bool
	EnableCR            bool
	EnableBucket        bool
	EnableLB            bool
	EnableVolume        bool
	EnableSG            bool
	EnableOrphanIP      bool
}

type ScalewaySessions struct {
	Cluster      *k8s.API
	Database     *rdb.API
	Namespace    *registry.API
	LoadBalancer *lb.ZonedAPI
	Instance     *instance.API
	Bucket       *minio.Client
	SG           *instance.API
}

type funcDeleteExpired func(sessions ScalewaySessions, options ScalewayOptions)

func RunPlecoScaleway(zones []string, interval int64, wg *sync.WaitGroup, options ScalewayOptions) {
	enabledRegions := make(map[string]string)
	for _, zone := range zones {
		if region := GetRegionfromZone(zone); region != "" {
			enabledRegions[region] = region
		}
		wg.Add(1)
		go runPlecoInZone(zone, interval, wg, options)
	}

	for _, enabledRegion := range enabledRegions {
		wg.Add(1)
		go runPlecoInRegion(enabledRegion, interval, wg, options)
	}
}

func runPlecoInZone(zone string, interval int64, wg *sync.WaitGroup, options ScalewayOptions) {
	defer wg.Done()
	scwZone := scw.Zone(zone)
	sessions := ScalewaySessions{}
	currentSession := CreateSessionWithZone(scwZone)
	options.Zone = scwZone.String()
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

	if options.EnableVolume {
		sessions.Instance = instance.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredVolumes)
	}

	if options.EnableSG {
		sessions.SG = instance.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteDetachedSecurityGroups)
	}

	if options.EnableCR {
		sessions.Namespace = registry.NewAPI(currentSession)
		sessions.Cluster = k8s.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteEmptyContainerRegistries)
	}

	if options.EnableOrphanIP {
		if sessions.LoadBalancer == nil {
			sessions.LoadBalancer = lb.NewZonedAPI(currentSession)
		}

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteOrphanIPAddresses)
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

func runPlecoInRegion(region string, interval int64, wg *sync.WaitGroup, options ScalewayOptions) {
	defer wg.Done()
	sessions := ScalewaySessions{}
	options.Region = scw.Region(region)
	currentSession := CreateSessionWithRegion(options.Region)

	var listServiceToCheckStatus []funcDeleteExpired
	if options.EnableBucket {
		sessions.Bucket = CreateMinIOSession(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredBuckets)
	}

	if options.EnableLB {
		sessions.Cluster = k8s.NewAPI(currentSession)
		sessions.LoadBalancer = lb.NewZonedAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredLBs)
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
