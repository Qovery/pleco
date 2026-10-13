package gcp

import (
	"context"
	"fmt"
	"time"

	runpb "cloud.google.com/go/run/apiv2/runpb"
	log "github.com/sirupsen/logrus"
)

// Job expires from its ttl and creation_date labels.
type Job struct {
	*runpb.Job
}

func (j Job) IsExpired() bool {
	// Jobs historically accept an explicit creation_date of zero as the Unix epoch.
	return j.Job != nil && isExpiredFromUnixCreation("job", j.Name, j.Labels["ttl"], j.Labels["creation_date"], true)
}

func DeleteExpiredJobs(sessions GCPSessions, options GCPOptions) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	jobsIterator := sessions.Job.ListJobs(ctx, &runpb.ListJobsRequest{
		Parent:      fmt.Sprintf("projects/%s/locations/%s", options.ProjectID, options.Location),
		ShowDeleted: false,
	})

	for {
		job, err := jobsIterator.Next()
		if err != nil {
			break
		}

		if !(Job{Job: job}).IsExpired() {
			continue
		}

		if options.DryRun {
			log.Info(fmt.Sprintf("Job `%s will be deleted`", job.Name))
			continue
		}

		// job is eligible to deletion
		log.Infof("Deleting job `%s` (creation_date `%s`, TTL `%s` seconds)", job.Name, job.Labels["creation_date"], job.Labels["ttl"])
		operation, err := sessions.Job.DeleteJob(ctx, &runpb.DeleteJobRequest{
			Name: job.Name,
		})
		if err != nil {
			log.Error(fmt.Sprintf("Error deleting job `%s`, error: %s", job.Name, err))
		}

		// this operation can be a bit long, we wait until it's done
		_, err = operation.Wait(ctx)
		if err != nil {
			log.Error(fmt.Sprintf("Error waiting for job `%s` deletion: %s", job.Name, err))
		}
	}
}
