package scaleway

import (
	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"strings"
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/registry/v1"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

func DeleteEmptyContainerRegistries(sessions ScalewaySessions, options ScalewayOptions) {
	emptyRegistries, _ := getEmptyRegistries(sessions.Namespace, &options)
	expiredRegirstries, _ := getExpiredRegistries(sessions.Cluster, sessions.Namespace, &options)
	deletableRegistriesIds := make(map[string]string)

	for _, emptyRegistry := range emptyRegistries {
		deletableRegistriesIds[emptyRegistry.ID] = emptyRegistry.ID
	}

	for _, expiredRegirstry := range expiredRegirstries {
		deletableRegistriesIds[expiredRegirstry] = expiredRegirstry
	}

	count, start := common.ElemToDeleteFormattedInfos("expired Scaleway namespace", len(deletableRegistriesIds), options.Region.String(), false)

	log.Info(count)

	if options.DryRun || len(deletableRegistriesIds) == 0 {
		return
	}

	log.Info(start)

	for _, deletableRegistryId := range deletableRegistriesIds {
		deleteRegistry(sessions.Namespace, deletableRegistryId, options.Region.String())
	}
}

func listRegistries(registryAPI *registry.API) ([]*registry.Namespace, string) {
	input := &registry.ListNamespacesRequest{PageSize: scw.Uint32Ptr(500)}
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
		if options.IsDestroyingCommand || (reg.ImageCount == 0 && reg.CreatedAt.UTC().Add(time.Hour).Before(time.Now().UTC())) && reg.Status != "deleting" {
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
		if options.IsDestroyingCommand || (registry.CreatedAt.UTC().Add(3*time.Hour).Before(time.Now().UTC()) && registry.Status != "deleting") {
			checkingRegistries[registry.Name] = registry.ID
		}
	}
	for _, cluster := range clusters {
		splitedName := strings.Split(cluster.Name, "-")
		checkingRegistries[splitedName[1]] = "keep-me"
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

func deleteRegistry(registryAPI *registry.API, registryId string, region string) {
	ns, err := registryAPI.DeleteNamespace(
		&registry.DeleteNamespaceRequest{
			Region:      scw.Region(region),
			NamespaceID: registryId,
		},
	)

	if err != nil {
		log.Errorf("Can't delete container registry %s in %s: %s", ns.Name, ns.Region, err.Error())
	} else {
		log.Debugf("Registry %s in %s deleted.", ns.Name, ns.Region)
	}
}
