package gcp

import (
	runpb "cloud.google.com/go/run/apiv2/runpb"
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

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

		ttlStr, ok := job.Labels["ttl"]
		if !ok || strings.TrimSpace(ttlStr) == "" {
			log.Info(fmt.Sprintf("No ttl label value found, ignoring this job (`%s`)", job.Name))
			continue
		}
		ttl, err := strconv.ParseInt(ttlStr, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this job (`%s`)", ttlStr, job.Name))
			continue
		}

		creationTimeStr := ""
		if job.Labels["creation_date"] != "" {
			creationTimeStr = job.Labels["creation_date"]
		} else {
			log.Info(fmt.Sprintf("No creation time value found, ignoring this job (`%s`)", job.Name))
			continue
		}
		creationTimeInt64, err := strconv.ParseInt(creationTimeStr, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("creation_date label value `%s` is not parsable to int64, ignoring this job (`%s`)", creationTimeStr, job.Name))
			continue
		}
		creationTime := time.Unix(creationTimeInt64, 0).UTC()

		// job is not expired (or is protected TTL = 0)
		if !ok || ttl == 0 || time.Now().UTC().Before(creationTime.UTC().Add(time.Second*time.Duration(ttl))) {
			continue
		}

		if options.DryRun {
			log.Info(fmt.Sprintf("Job `%s will be deleted`", job.Name))
			continue
		}

		// job is eligible to deletion
		log.Info(fmt.Sprintf("Deleting job `%s` created at `%s` UTC (TTL `{%d}` seconds)", job.Name, creationTimeStr, ttl))
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
