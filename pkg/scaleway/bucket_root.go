package scaleway

import (
	"context"
	"github.com/Qovery/pleco/pkg/common"
	"github.com/minio/minio-go/v7"
	"github.com/scaleway/scaleway-sdk-go/scw"
	log "github.com/sirupsen/logrus"
	"time"
)

type ScalewatBucket struct {
	Name         string
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
	ObjectsInfos []minio.ObjectInfo
}

func DeleteExpiredBuckets(sessions ScalewaySessions, options ScalewayOptions) {
	expiredBuckets := getExpiredBuckets(sessions.Bucket, options.TagName, options.Region)

	count, start := common.ElemToDeleteFormattedInfos("expired bucket", len(expiredBuckets), string(options.Region))

	log.Debug(count)

	if options.DryRun || len(expiredBuckets) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredBucket := range expiredBuckets {
		deleteBucket(sessions.Bucket, expiredBucket)
	}
}

func listBuckets(bucketApi *minio.Client, tagName string, region scw.Region) []ScalewatBucket {
	ctx := context.Background()
	buckets, err := bucketApi.ListBuckets(ctx)
	if err != nil {
		log.Errorf("Can't list bucket for region %s: %s", region, err.Error())
		return []ScalewatBucket{}
	}

	scwBuckets := []ScalewatBucket{}
	for _, bucket := range buckets {
		objectsInfos := listBucketObjects(bucketApi, ctx, bucket.Name)

		creationDate, _ := time.Parse(time.RFC3339, bucket.CreationDate.Format(time.RFC3339))
		scwBuckets = append(scwBuckets, ScalewatBucket{
			Name:         bucket.Name,
			CreationDate: creationDate,
			TTL:          0,
			IsProtected:  false,
			ObjectsInfos: objectsInfos,
		})
	}

	return scwBuckets
}

func listBucketObjects(bucketApi *minio.Client, ctx context.Context, bucketName string) []minio.ObjectInfo {
	objects := bucketApi.ListObjects(ctx, bucketName, minio.ListObjectsOptions{})
	objectsInfos := []minio.ObjectInfo{}
	for object := range objects {
		objectsInfos = append(objectsInfos, object)
	}

	return objectsInfos
}

func getExpiredBuckets(bucketApi *minio.Client, tagName string, region scw.Region) []ScalewatBucket {
	buckets := listBuckets(bucketApi, tagName, region)

	expiredBuckets := []ScalewatBucket{}
	for _, bucket := range buckets {
		if len(bucket.ObjectsInfos) == 0 && bucket.CreationDate.Add(168*time.Hour).Before(time.Now()) {
			expiredBuckets = append(expiredBuckets, bucket)
		}
	}

	return expiredBuckets
}

func deleteBucket(bucketApi *minio.Client, bucket ScalewatBucket) {
	err := bucketApi.RemoveBucket(context.Background(), bucket.Name)
	if err != nil {
		log.Errorf("Can't delete bucket %s: %s", bucket.Name, err.Error())
	}
}
