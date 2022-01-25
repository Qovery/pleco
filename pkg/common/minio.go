package common

import (
	"context"
	"github.com/minio/minio-go/v7"
	log "github.com/sirupsen/logrus"
	"time"
)

type MinioBucket struct {
	Name         string
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
	ObjectsInfos []minio.ObjectInfo
}

func listBuckets(bucketApi *minio.Client, tagName string, region string) []MinioBucket {
	ctx := context.Background()
	buckets, err := bucketApi.ListBuckets(ctx)
	if err != nil {
		log.Errorf("Can't list bucket for region %s: %s", region, err.Error())
		return []MinioBucket{}
	}

	scwBuckets := []MinioBucket{}
	for _, bucket := range buckets {
		objectsInfos := ListBucketObjects(bucketApi, ctx, bucket.Name)

		creationDate, _ := time.Parse(time.RFC3339, bucket.CreationDate.Format(time.RFC3339))
		scwBuckets = append(scwBuckets, MinioBucket{
			Name:         bucket.Name,
			CreationDate: creationDate,
			TTL:          0,
			IsProtected:  false,
			ObjectsInfos: objectsInfos,
		})
	}

	return scwBuckets
}

func ListBucketObjects(bucketApi *minio.Client, ctx context.Context, bucketName string) []minio.ObjectInfo {
	objects := bucketApi.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})
	objectsInfos := []minio.ObjectInfo{}
	for object := range objects {
		objectsInfos = append(objectsInfos, object)
	}

	return objectsInfos
}

func GetExpiredBuckets(bucketApi *minio.Client, tagName string, region string) []MinioBucket {
	buckets := listBuckets(bucketApi, tagName, region)

	expiredBuckets := []MinioBucket{}
	for _, bucket := range buckets {
		if bucket.CreationDate.UTC().Add(2 * time.Hour).Before(time.Now().UTC()) {
			expiredBuckets = append(expiredBuckets, bucket)
		}
	}

	return expiredBuckets
}

func EmptyBucket(bucketApi *minio.Client, bucketName string, objects []minio.ObjectInfo) {
	for _, object := range objects {
		err := bucketApi.RemoveObject(context.TODO(), bucketName, object.Key, minio.RemoveObjectOptions{ForceDelete: true})
		if err != nil {
			log.Errorf("Can't delete object %s for bucket %s: %s", object.Key, bucketName, err.Error())
		}
	}

}

func DeleteBucket(bucketApi *minio.Client, bucket MinioBucket) {
	err := bucketApi.RemoveBucket(context.Background(), bucket.Name)
	if err != nil {
		log.Errorf("Can't delete bucket %s: %s", bucket.Name, err.Error())
	}
}
