package gcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
)

const bucketDeleteWorkers = 3
const bucketDeleteRatePerSec = 10
const bucketDeleteMaxRetries = 6

func deleteObjectWithRetry(bucketHandle *storage.BucketHandle, bucketName, objectName string, generation int64) error {
	backoff := 500 * time.Millisecond
	for attempt := 0; attempt <= bucketDeleteMaxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
		err := bucketHandle.Object(objectName).Generation(generation).Delete(ctx)
		cancel()
		if err == nil {
			return nil
		}

		if apiErr, ok := err.(*googleapi.Error); ok && (apiErr.Code == 429 || apiErr.Code == 500 || apiErr.Code == 503) {
			if attempt == bucketDeleteMaxRetries {
				return err
			}
			log.Debug(fmt.Sprintf("Retrying object `%s` in bucket `%s` after %s (attempt %d/%d): %s", objectName, bucketName, backoff, attempt+1, bucketDeleteMaxRetries, err))
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		return err
	}
	return nil
}

func emptyBucket(bucketHandle *storage.BucketHandle, bucketName string) bool {
	type objectVersion struct {
		name       string
		generation int64
	}

	var objects []objectVersion
	it := bucketHandle.Objects(context.Background(), &storage.Query{Versions: true})
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Error(fmt.Sprintf("Error listing objects in bucket `%s`, error: %s", bucketName, err))
			return false
		}
		objects = append(objects, objectVersion{name: obj.Name, generation: obj.Generation})
	}

	if len(objects) == 0 {
		return true
	}

	log.Info(fmt.Sprintf("Emptying bucket `%s`: deleting %d object versions with %d workers at %d req/s", bucketName, len(objects), bucketDeleteWorkers, bucketDeleteRatePerSec))

	limiter := rate.NewLimiter(rate.Limit(bucketDeleteRatePerSec), bucketDeleteWorkers)

	jobs := make(chan objectVersion, len(objects))
	for _, obj := range objects {
		jobs <- obj
	}
	close(jobs)

	var mu sync.Mutex
	emptied := true

	var wg sync.WaitGroup
	for w := 0; w < bucketDeleteWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for obj := range jobs {
				if err := limiter.Wait(context.Background()); err != nil {
					log.Error(fmt.Sprintf("Rate limiter error for bucket `%s`: %s", bucketName, err))
					mu.Lock()
					emptied = false
					mu.Unlock()
					continue
				}
				if err := deleteObjectWithRetry(bucketHandle, bucketName, obj.name, obj.generation); err != nil {
					log.Error(fmt.Sprintf("Error deleting object `%s` (generation %d) from bucket `%s`, error: %s", obj.name, obj.generation, bucketName, err))
					mu.Lock()
					emptied = false
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	return emptied
}

func DeleteExpiredBuckets(sessions GCPSessions, options GCPOptions) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	var bucketsIterator = sessions.Bucket.Buckets(ctx, options.ProjectID)
	for {
		bucket, err := bucketsIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Error(fmt.Sprintf("Error listing buckets for project `%s`, error: %s", options.ProjectID, err))
			break
		}

		if !strings.EqualFold(bucket.Location, options.Location) {
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
		if ttl == 0 || time.Now().UTC().Before(bucket.Created.UTC().Add(time.Second*time.Duration(ttl))) {
			continue
		}

		if options.DryRun {
			log.Info(fmt.Sprintf("Bucket `%s` will be deleted", bucket.Name))
			continue
		}

		log.Info(fmt.Sprintf("Deleting bucket `%s` created at `%s` UTC (TTL `%d` seconds)", bucket.Name, bucket.Created.UTC(), ttl))

		if !emptyBucket(sessions.Bucket.Bucket(bucket.Name), bucket.Name) {
			log.Warn(fmt.Sprintf("Skipping deletion of bucket `%s`: failed to empty it", bucket.Name))
			continue
		}

		ctxDeleteBucket, cancelDeleteBucket := context.WithTimeout(context.Background(), time.Second*60)
		if err := sessions.Bucket.Bucket(bucket.Name).Delete(ctxDeleteBucket); err != nil {
			log.Error(fmt.Sprintf("Error deleting bucket `%s`, error: %s", bucket.Name, err))
		}
		cancelDeleteBucket()
	}
}
