package azure

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armcontainerregistry "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerregistry/armcontainerregistry"
	armresources "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	armstorage "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/sirupsen/logrus"
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
	EnableStorageAccount bool
	EnableACR           bool
}

type AzureSessions struct {
	RG             *armresources.ResourceGroupsClient
	StorageAccount *armstorage.AccountsClient
	ACR            *armcontainerregistry.RegistriesClient
}

type funcDeleteExpired func(sessions AzureSessions, options AzureOptions)

// Initialize creates and returns an AzureSessions object with authenticated clients
func Initialize() (AzureSessions, error) {
	var sessions AzureSessions
	
	// Get subscription ID from environment variable
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		return sessions, fmt.Errorf("AZURE_SUBSCRIPTION_ID environment variable is not set")
	}

	// Create a credential using the DefaultAzureCredential
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return sessions, fmt.Errorf("failed to create credential: %v", err)
	}

	// Initialize Resource Groups client
	rgClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return sessions, fmt.Errorf("failed to create resource groups client: %v", err)
	}
	sessions.RG = rgClient

	// Initialize Storage Account client
	storageClient, err := armstorage.NewAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return sessions, fmt.Errorf("failed to create storage account client: %v", err)
	}
	sessions.StorageAccount = storageClient

	// Initialize Container Registry client
	acrClient, err := armcontainerregistry.NewRegistriesClient(subscriptionID, cred, nil)
	if err != nil {
		return sessions, fmt.Errorf("failed to create container registry client: %v", err)
	}
	sessions.ACR = acrClient

	return sessions, nil
}

func RunPlecoAzure(locations []string, interval int64, wg *sync.WaitGroup, options AzureOptions) {
	for _, location := range locations {
		wg.Add(1)
		go runPlecoInRegion(location, interval, wg, options)
	}
}

func runPlecoInRegion(location string, interval int64, wg *sync.WaitGroup, options AzureOptions) {
	defer wg.Done()
	options.Location = location
	
	// Initialize Azure sessions with authentication
	sessions, err := Initialize()
	if err != nil {
		logrus.Errorf("Failed to initialize Azure sessions for location %s: %v", location, err)
		return
	}

	logrus.Infof("Starting to check expired resources in location %s.", options.Location)

	var listServiceToCheckStatus []funcDeleteExpired

	if options.EnableRG {
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredRGs)
	}
	
	if options.EnableStorageAccount {
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredStorageAccounts)
	}

	if options.EnableACR {
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredACRs)
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
