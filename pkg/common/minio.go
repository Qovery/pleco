package common

import (
	"context"
	"github.com/minio/minio-go/v7"
	log "github.com/sirupsen/logrus"
	"time"
)

type MinioBucket struct {
	CloudProviderResource
	Name         string
	ObjectsInfos []minio.ObjectInfo
}

func listBuckets(bucketApi *minio.Client, tagName string, region string, withTags bool) []MinioBucket {
	ctx := context.Background()
	buckets, err := bucketApi.ListBuckets(ctx)
	if err != nil {
		log.Errorf("Can't list bucket for region %s: %s", region, err.Error())
		return []MinioBucket{}
	}

	bucketLimit := 50
	if len(buckets) < bucketLimit {
		bucketLimit = len(buckets)
	}

	scwBuckets := []MinioBucket{}
	for _, bucket := range buckets[:bucketLimit] {
		objectsInfos := ListBucketObjects(bucketApi, ctx, bucket.Name)
		essentialTags := EssentialTags{}
		if withTags {
			bucketTags := listBucketTags(bucketApi, context.TODO(), bucket.Name)
			essentialTags = GetEssentialTags(bucketTags, tagName)
		}
		creationDate, _ := time.Parse(time.RFC3339, bucket.CreationDate.Format(time.RFC3339))
		scwBuckets = append(scwBuckets, MinioBucket{
			CloudProviderResource: CloudProviderResource{
				Identifier:   bucket.Name,
				Description:  "Bucket: " + bucket.Name,
				CreationDate: creationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  false,
			},
			ObjectsInfos: objectsInfos,
		})
	}

	return scwBuckets
}

func listBucketTags(bucketApi *minio.Client, ctx context.Context, bucketName string) []string {
	objects, err := bucketApi.GetBucketTagging(ctx, bucketName)
	tags := []string{}
	if err != nil {
		log.Errorf("Can't get tags for bucket %s: %s", bucketName, err.Error())
		return tags
	}

	for _, value := range objects.ToMap() {
		tags = append(tags, value)
	}

	return tags
}

func ListBucketObjects(bucketApi *minio.Client, ctx context.Context, bucketName string) []minio.ObjectInfo {
	objects := bucketApi.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})
	objectsInfos := []minio.ObjectInfo{}
	for object := range objects {
		objectsInfos = append(objectsInfos, object)
	}

	return objectsInfos
}

func GetExpiredBuckets(bucketApi *minio.Client, tagName string, region string, tagValue string) []MinioBucket {
	buckets := listBuckets(bucketApi, tagName, region, true)

	expiredBuckets := []MinioBucket{}
	for _, bucket := range buckets {
		if bucket.IsResourceExpired(tagValue) {
			expiredBuckets = append(expiredBuckets, bucket)
		}
	}

	return expiredBuckets
}

func GetUnusedBuckets(bucketApi *minio.Client, tagName string, region string) []MinioBucket {
	buckets := listBuckets(bucketApi, tagName, region, false)

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
	err := bucketApi.RemoveBucket(context.Background(), bucket.Identifier)
	if err != nil {
		log.Errorf("Can't delete bucket %s: %s", bucket.Identifier, err.Error())
	}
}
