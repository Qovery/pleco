package gcp

import (
	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"fmt"
	log "github.com/sirupsen/logrus"
	"go.uber.org/ratelimit"
	"golang.org/x/net/context"
	"strconv"
	"strings"
	"time"
)

type RepositoryToDelete struct {
	Name         string
	TTL          int64
	CreationTime time.Time
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

			ttlStr, ok := repository.Labels["ttl"]
			if !ok || strings.TrimSpace(ttlStr) == "" {
				log.Info(fmt.Sprintf("No ttl label value found, ignoring this repository (`%s`)", repository.Name))
				continue
			}
			ttl, err := strconv.ParseInt(ttlStr, 10, 64)
			if err != nil {
				log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this repository (`%s`)", ttlStr, repository.Name))
				continue
			}
			creationTime := repository.CreateTime.AsTime()
			// repository is not expired (or is protected TTL = 0)
			if !ok || ttl == 0 || time.Now().UTC().Before(creationTime.UTC().Add(time.Second*time.Duration(ttl))) {
				continue
			}

			if options.DryRun {
				log.Info(fmt.Sprintf("Repository `%s will be deleted`", repository.Name))
				continue
			}

			// repository is eligible to deletion
			log.Info(fmt.Sprintf("Deleting repository `%s` created at `%s` UTC (TTL `{%d}` seconds)", repository.Name, creationTime.UTC(), ttl))

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
