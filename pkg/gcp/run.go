package gcp

import (
	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	compute "cloud.google.com/go/compute/apiv1"
	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/storage"
	"encoding/base64"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	iam "google.golang.org/api/iam/v1"
	"os"
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
}

type GCPSessions struct {
	Bucket           *storage.Client
	ArtifactRegistry *artifactregistry.Client
	Cluster          *container.ClusterManagerClient
	Network          *compute.NetworksClient
	IAM              *iam.Service
}

type funcDeleteExpired func(sessions GCPSessions, options GCPOptions)

func RunPlecoGCP(regions []string, interval int64, wg *sync.WaitGroup, options GCPOptions) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON") == "" {
		jsonB64EncodedCredentialsEnv := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON_BASE64")
		if jsonB64EncodedCredentialsEnv != "" {
			decodedCredentialsEnv, err := base64.StdEncoding.DecodeString(jsonB64EncodedCredentialsEnv)
			if err != nil {
				logrus.Errorf("GOOGLE_APPLICATION_CREDENTIALS_JSON_BASE64 cannot be base64 decoded: %s", err)
				return
			}
			if os.Setenv("GOOGLE_APPLICATION_CREDENTIALS_JSON", string(decodedCredentialsEnv)) != nil {
				logrus.Errorf("GOOGLE_APPLICATION_CREDENTIALS_JSON cannot be set: %s", err)
				return
			}
		}
	}
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

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredVPCs)
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
