package gcp

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"strconv"
	"strings"
	"time"
)

func DeleteExpiredBuckets(sessions GCPSessions, options GCPOptions) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	var bucketsIterator = sessions.Bucket.Buckets(ctx, options.ProjectID)
	for {
		bucket, err := bucketsIterator.Next()
		if err != nil {
			break
		}

		// bucket from another region
		if strings.EqualFold(bucket.Location, options.Location) {
			continue
		}

		ttlStr, ok := bucket.Labels["ttl"]
		if !ok || strings.TrimSpace(ttlStr) == "" {
			log.Info(fmt.Sprintf("No ttl label value found, ignoring this bucket (`%s`)", bucket.Name))
			continue
		}
		ttl, err := strconv.ParseInt(ttlStr, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this bucket (`%s`)", ttlStr, bucket.Name))
			continue
		}
		// bucket is not expired (or is protected TTL = 0)
		if !ok || ttl == 0 || time.Now().UTC().Before(bucket.Created.UTC().Add(time.Second*time.Duration(ttl))) {
			continue
		}

		if options.DryRun {
			log.Info(fmt.Sprintf("Bucket `%s will be deleted`", bucket.Name))
			continue
		}

		// bucket is eligible to deletion
		objectsIterator := sessions.Bucket.Bucket(bucket.Name).Objects(ctx, nil)
		for {
			object, err := objectsIterator.Next()
			if err != nil {
				break
			}

			err = sessions.Bucket.Bucket(bucket.Name).Object(object.Name).Delete(ctx)
			if err != nil {
				log.Error(fmt.Sprintf("Error deleting object `%s` from bucket `%s`, error: %s", object.Name, bucket.Name, err))
			}
		}

		log.Info(fmt.Sprintf("Deleting bucket `%s` created at `%s` UTC (TTL `{%d}` seconds)", bucket.Name, bucket.Created.UTC(), ttl))
		if err := sessions.Bucket.Bucket(bucket.Name).Delete(ctx); err != nil {
			log.Error(fmt.Sprintf("Error deleting bucket `%s`, error: %s", bucket.Name, err))
		}
	}
}
