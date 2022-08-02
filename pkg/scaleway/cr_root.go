package scaleway

import (
	"fmt"
	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	"strings"
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/registry/v1"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

func DeleteEmptyContainerRegistries(sessions ScalewaySessions, options ScalewayOptions) {
	emptyRegistries, _ := getEmptyRegistries(sessions.Namespace, &options)
	expiredRegirstries, _ := getExpiredRegistries(sessions.Cluster, sessions.Namespace, &options)
	deletableRegistries := make(map[string]string)

	for _, emptyRegistry := range emptyRegistries {
		deletableRegistries[emptyRegistry.ID] = emptyRegistry.ID
	}

	for _, expiredRegirstry := range expiredRegirstries {
		deletableRegistries[expiredRegirstry] = expiredRegirstry
	}

	count, start := common.ElemToDeleteFormattedInfos("expired Scaleway namespace", len(deletableRegistries), options.Zone, true)

	log.Debug(count)

	if options.DryRun || len(emptyRegistries) == 0 {
		return
	}

	log.Debug(start)

	for _, emptyRegistry := range emptyRegistries {
		deleteRegistry(sessions.Namespace, emptyRegistry.ID)
	}

	for _, expiredRegistryId := range expiredRegirstries {
		deleteRegistry(sessions.Namespace, expiredRegistryId)
	}
}

func listRegistries(registryAPI *registry.API) ([]*registry.Namespace, string) {
	input := &registry.ListNamespacesRequest{}
	result, err := registryAPI.ListNamespaces(input)
	if err != nil {
		log.Errorf("Can't list container registries for region %s: %s", input.Region, err.Error())
		return []*registry.Namespace{}, input.Region.String()
	}

	return result.Namespaces, input.Region.String()
}

func getEmptyRegistries(registryAPI *registry.API, options *ScalewayOptions) ([]*registry.Namespace, string) {
	registries, region := listRegistries(registryAPI)

	emptyRegistries := []*registry.Namespace{}
	for _, reg := range registries {
		if options.IsDestroyingCommand || (reg.ImageCount == 0 && reg.CreatedAt.UTC().Add(time.Hour).Before(time.Now().UTC())) {
			emptyRegistries = append(emptyRegistries, reg)
		}
	}

	return emptyRegistries, region
}

func getExpiredRegistries(clusterAPI *k8s.API, registryAPI *registry.API, options *ScalewayOptions) ([]string, string) {
	clusters, _ := ListClusters(clusterAPI, options.TagName)
	registries, region := listRegistries(registryAPI)

	checkingRegistries := make(map[string]string)
	for _, registry := range registries {
		if options.IsDestroyingCommand || registry.CreatedAt.UTC().Add(4*time.Hour).Before(time.Now().UTC()) {
			checkingRegistries[registry.ID] = registry.ID
		}
	}
	for _, cluster := range clusters {
		splitedName := strings.Split(cluster.Name, "-")
		configName := fmt.Sprintf("%s-kubeconfigs-%s", splitedName[0], splitedName[1])
		logsName := fmt.Sprintf("%s-logs-%s", splitedName[0], splitedName[1])
		checkingRegistries[configName] = "keep-me"
		checkingRegistries[logsName] = "keep-me"
	}

	expiredRegistriesId := []string{}
	for _, registryId := range checkingRegistries {
		// do we need to force delete every bucket on detroy command ?
		if !strings.Contains(registryId, "keep-me") {
			expiredRegistriesId = append(expiredRegistriesId, registryId)
		}
	}

	return expiredRegistriesId, region
}

func deleteRegistry(registryAPI *registry.API, registryId string) {
	_, err := registryAPI.DeleteNamespace(
		&registry.DeleteNamespaceRequest{
			NamespaceID: registryId,
		},
	)

	if err != nil {
		log.Errorf("Can't delete container registry with Id %s: %s", registryId, err.Error())
	}
}
