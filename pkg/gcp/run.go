package gcp

import (
	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	compute "cloud.google.com/go/compute/apiv1"
	container "cloud.google.com/go/container/apiv1"
	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	iam "google.golang.org/api/iam/v1"
	"sync"
	"time"
)

type GCPOptions struct {
	ProjectID              string
	TagName                string
	TagValue               string
	DisableTTLCheck        bool
	IsDestroyingCommand    bool
	DryRun                 bool
	Location               string
	EnableCluster          bool
	EnableBucket           bool
	EnableNetwork          bool
	EnableArtifactRegistry bool
	EnableIAM              bool
	EnableRouter           bool
	EnableJob              bool
}

type GCPSessions struct {
	Bucket           *storage.Client
	ArtifactRegistry *artifactregistry.Client
	Cluster          *container.ClusterManagerClient
	Network          *compute.NetworksClient
	Subnetwork       *compute.SubnetworksClient
	Router           *compute.RoutersClient
	IAM              *iam.Service
	Job              *run.JobsClient
}

type funcDeleteExpired func(sessions GCPSessions, options GCPOptions)

func RunPlecoGCP(regions []string, interval int64, wg *sync.WaitGroup, options GCPOptions) {
	for _, region := range regions {
		wg.Add(1)
		go runPlecoInRegion(region, interval, wg, options)
	}
}

func runPlecoInRegion(location string, interval int64, wg *sync.WaitGroup, options GCPOptions) {
	defer wg.Done()
	options.Location = location
	sessions := GCPSessions{}

	logrus.Infof("Starting to check expired resources in region %s.", options.Location)

	var listServiceToCheckStatus []funcDeleteExpired
	ctx, _ := context.WithTimeout(context.Background(), time.Second*30)

	if options.EnableBucket {
		client, err := storage.NewClient(ctx)
		if err != nil {
			logrus.Errorf("storage.NewClient: %s", err)
			return
		}
		defer client.Close()
		sessions.Bucket = client

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredBuckets)
	}

	if options.EnableArtifactRegistry {
		client, err := artifactregistry.NewClient(ctx)
		if err != nil {
			logrus.Errorf("artifactregistry.NewClient: %s", err)
			return
		}
		defer client.Close()
		sessions.ArtifactRegistry = client

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredArtifactRegistryRepositories)
	}

	if options.EnableCluster {
		client, err := container.NewClusterManagerClient(ctx)
		if err != nil {
			logrus.Errorf("container.NewClusterManagerClient: %s", err)
			return
		}
		defer client.Close()
		sessions.Cluster = client

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredGKEClusters)
	}

	if options.EnableNetwork {
		networkClient, err := compute.NewNetworksRESTClient(ctx)
		if err != nil {
			logrus.Errorf("compute.NewNetworksRESTClient: %s", err)
			return
		}
		defer networkClient.Close()
		sessions.Network = networkClient

		subnetworkClient, err := compute.NewSubnetworksRESTClient(ctx)
		if err != nil {
			logrus.Errorf("compute.NewSubnetworksRESTClient: %s", err)
			return
		}
		defer subnetworkClient.Close()
		sessions.Subnetwork = subnetworkClient

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredVPCs)
	}

	if options.EnableRouter {
		routerClient, err := compute.NewRoutersRESTClient(ctx)
		if err != nil {
			logrus.Errorf("compute.NewRoutersRESTClient: %s", err)
			return
		}
		defer routerClient.Close()
		sessions.Router = routerClient

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredRouters)
	}

	if options.EnableIAM {
		iamService, err := iam.NewService(ctx)
		if err != nil {
			logrus.Errorf("iam.NewService: %s", err)
			return
		}
		sessions.IAM = iamService

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredServiceAccounts)
	}

	if options.EnableJob {
		jobClient, err := run.NewJobsClient(ctx)
		if err != nil {
			logrus.Errorf("run.NewJobsClient: %s", err)
			return
		}
		defer jobClient.Close()
		sessions.Job = jobClient

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredJobs)
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
