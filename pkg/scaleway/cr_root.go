package scaleway

import (
	"github.com/scaleway/scaleway-sdk-go/api/registry/v1"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/Qovery/pleco/pkg/common"
)

func DeleteEmptyContainerRegistries(sessions ScalewaySessions, options ScalewayOptions) {
	emptyRegistries, _ := getEmptyRegistries(sessions.Namespace, &options)

	count, start := common.ElemToDeleteFormattedInfos("empty Scaleway namespace", len(emptyRegistries), options.Zone, true)

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

func getEmptyRegistries(registryAPI *registry.API, options *ScalewayOptions) ([]*registry.Namespace, string) {
	registries, region := listRegistries(registryAPI)

	emptyRegistries := []*registry.Namespace{}
	for _, reg := range registries {
		if reg.ImageCount == 0 &&
			(options.IsDestroyingCommand && reg.CreatedAt.UTC().Add(time.Hour).After(time.Now().UTC())) {
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
