package scaleway

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/scaleway/scaleway-sdk-go/api/registry/v1"
	log "github.com/sirupsen/logrus"
	"time"
)

func DeleteEmptyContainerRegistries(sessions *ScalewaySessions, options *ScalewayOptions) {
	emptyRegistries, region := getEmptyRegistries(sessions.Namespace)

	count, start := common.ElemToDeleteFormattedInfos("empty Scaleway namespace", len(emptyRegistries), region)

	log.Debug(count)

	if options.DryRun || len(emptyRegistries) == 0 {
		return
	}

	log.Debug(start)

	for _, emptyRegistry := range emptyRegistries {
		deleteRegistry(sessions.Namespace, emptyRegistry)
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

func getEmptyRegistries(registryAPI *registry.API) ([]*registry.Namespace, string) {
	registries, region := listRegistries(registryAPI)

	emptyRegistries := []*registry.Namespace{}
	for _, reg := range registries {
		if reg.ImageCount == 0 && reg.CreatedAt.Add(time.Hour).After(time.Now()) {
			emptyRegistries = append(emptyRegistries, reg)
		}
	}

	return emptyRegistries, region
}

func deleteRegistry(registryAPI *registry.API, reg *registry.Namespace) {
	_, err := registryAPI.DeleteNamespace(
		&registry.DeleteNamespaceRequest{
			NamespaceID: reg.ID,
		},
	)

	if err != nil {
		log.Errorf("Can't delete container registry %s: %s", reg.Name, err.Error())
	}
}
