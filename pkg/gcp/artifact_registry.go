package gcp

import (
	"fmt"
	"time"

	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	log "github.com/sirupsen/logrus"
	"go.uber.org/ratelimit"
	"golang.org/x/net/context"
)

// ArtifactRegistryRepository expires from its ttl label and native creation timestamp.
type ArtifactRegistryRepository struct {
	*artifactregistrypb.Repository
}

func (r ArtifactRegistryRepository) IsExpired() bool {
	return r.Repository != nil && isExpiredFromCreationTime("repository", r.Name, r.Labels["ttl"], r.CreateTime.AsTime())
}

func DeleteExpiredArtifactRegistryRepositories(sessions GCPSessions, options GCPOptions) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	limiter := ratelimit.New(1, ratelimit.Per(1*time.Second))
	var pageToken = ""
	for {
		var repositoriesIterator = sessions.ArtifactRegistry.ListRepositories(ctx, &artifactregistrypb.ListRepositoriesRequest{Parent: fmt.Sprintf("projects/%s/locations/%s", options.ProjectID, options.Location), PageToken: pageToken, PageSize: 100})

		for {
			repository, err := repositoriesIterator.Next()
			if err != nil {
				break
			}

			if !(ArtifactRegistryRepository{Repository: repository}).IsExpired() {
				continue
			}

			if options.DryRun {
				log.Info(fmt.Sprintf("Repository `%s will be deleted`", repository.Name))
				continue
			}

			// repository is eligible to deletion
			log.Info(fmt.Sprintf("Deleting repository `%s` created at `%s` UTC (TTL `%s` seconds)", repository.Name, repository.CreateTime.AsTime().UTC(), repository.Labels["ttl"]))

			// wait for one available slot for deletion
			ctxDelete, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()
			limiter.Take()
			if _, err := sessions.ArtifactRegistry.DeleteRepository(ctxDelete, &artifactregistrypb.DeleteRepositoryRequest{
				Name: repository.Name,
			}); err != nil {
				log.Error(fmt.Sprintf("Error deleting repository `%s`, error: %s", repository.Name, err))
			}
		}

		var pageInfo = repositoriesIterator.PageInfo()
		if pageInfo != nil {
			pageToken = repositoriesIterator.PageInfo().Token
		}
		if pageInfo == nil || pageInfo.Remaining() == 0 {
			break
		}
	}
}
