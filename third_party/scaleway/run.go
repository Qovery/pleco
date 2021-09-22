package scaleway

import (
	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/api/rdb/v1"
	"github.com/scaleway/scaleway-sdk-go/api/registry/v1"
	"github.com/scaleway/scaleway-sdk-go/api/vpc/v1"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type ScalewayOption struct {
	TagName        string
	DryRun         bool
	EnableCluster  bool
	EnableInstance bool
	EnableDB       bool
	EnableLB       bool
	EnableCR       bool
	EnableVPC      bool
}

type ScalewaySessions struct {
	Cluster      *k8s.API
	Database     *rdb.API
	Instance     *instance.API
	LoadBalancer *lb.API
	Namespace    *registry.API
	VPC          *vpc.API
}

type funcDeleteExpired func(sessions *ScalewaySessions, options *ScalewayOption)

func RunPlecoScaleway(interval int64, wg *sync.WaitGroup, options *ScalewayOption) {
	defer wg.Done()

	sessions := &ScalewaySessions{}
	currentSession := CreateSession()
	organization, _ := currentSession.GetDefaultOrganizationID()

	logrus.Infof("Starting to check expired resources for organization %s.", organization)

	var listServiceToCheckStatus []funcDeleteExpired

	if options.EnableCluster {
		sessions.Cluster = k8s.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredClusters)
	}

	if options.EnableDB {
		sessions.Database = rdb.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredDatabases)
	}

	if options.EnableLB {
		sessions.LoadBalancer = lb.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredLoadBalancers)
	}

	if options.EnableCR {
		sessions.Namespace = registry.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredNamespaces)
	}

	if options.EnableVPC {
		sessions.VPC = vpc.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredVPCs)
	}

	if options.EnableInstance {
		sessions.Instance = instance.NewAPI(currentSession)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredInstances)
	}

	for _, check := range listServiceToCheckStatus {
		check(sessions, options)
	}

	time.Sleep(time.Duration(interval) * time.Second)
}
