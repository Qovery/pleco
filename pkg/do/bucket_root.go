package do

import (
	"fmt"
	"github.com/digitalocean/godo"
	"github.com/minio/minio-go/v7"
	log "github.com/sirupsen/logrus"
	"strings"

	"github.com/Qovery/pleco/pkg/common"
)

func DeleteExpiredBuckets(sessions DOSessions, options DOOptions) {
	expiredBuckets := emptyBuckets(sessions.Client, sessions.Bucket, options.TagName, options.Region, options.DryRun)

	count, start := common.ElemToDeleteFormattedInfos("expired bucket", len(expiredBuckets), options.Region)

	log.Debug(count)

	if options.DryRun || len(expiredBuckets) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredBucket := range expiredBuckets {
		common.DeleteBucket(sessions.Bucket, expiredBucket)
	}
}

func emptyBuckets(doApi *godo.Client, bucketApi *minio.Client, tagName string, region string, dryRun bool) []common.MinioBucket {
	buckets := getBucketsToEmpty(doApi, bucketApi, tagName, region)

	for _, bucket := range buckets {
		if !dryRun {
			common.EmptyBucket(bucketApi, bucket.Name, bucket.ObjectsInfos)
		}
	}

	return buckets
}

func getBucketsToEmpty(doApi *godo.Client, bucketApi *minio.Client, tagName string, region string) []common.MinioBucket {
	buckets := common.GetUnusedBuckets(bucketApi, tagName, region)
	clusters := listClusters(doApi, tagName, region)
	_, _ = buckets, clusters

	checkingBuckets := make(map[string]common.MinioBucket)
	for _, bucket := range buckets {
		checkingBuckets[bucket.Name] = bucket
	}

	for _, cluster := range clusters {
		splitedName := strings.Split(cluster.Name, "-")
		configName := fmt.Sprintf("%s-kubeconfigs-%s", splitedName[0], splitedName[1])
		logsName := fmt.Sprintf("%s-logs-%s", splitedName[0], splitedName[1])
		checkingBuckets[configName] = common.MinioBucket{Name: "keep-me"}
		checkingBuckets[logsName] = common.MinioBucket{Name: "keep-me"}
	}

	emptyBuckets := []common.MinioBucket{}
	for _, bucket := range checkingBuckets {
		// do we need to force delete every bucket on detroy command ?
		if !strings.Contains(bucket.Name, "keep-me") {
			emptyBuckets = append(emptyBuckets, bucket)
		}
	}

	return emptyBuckets
}
