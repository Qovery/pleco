package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

func listTaggedBuckets(s3Session s3.S3, tagName string) ([]common.CloudProviderResource, error) {
	var taggedS3Buckets []common.CloudProviderResource
	currentRegion := s3Session.Config.Region

	result, bucketErr := s3Session.ListBuckets(&s3.ListBucketsInput{})
	if bucketErr != nil {
		return nil, bucketErr
	}

	if len(result.Buckets) == 0 {
		return nil, nil
	}

	for _, bucket := range result.Buckets {
		location, locationErr := s3Session.GetBucketLocation(
			&s3.GetBucketLocationInput{
				Bucket: aws.String(*bucket.Name),
			})

		if locationErr != nil {
			log.Errorf("Location error for bucket %s: %s", *bucket.Name, locationErr.Error())
			continue
		}

		if location.LocationConstraint == nil || *location.LocationConstraint != *currentRegion {
			continue
		}

		bucketTags, tagErr := s3Session.GetBucketTagging(
			&s3.GetBucketTaggingInput{
				Bucket: aws.String(*bucket.Name),
			})

		if tagErr != nil && !strings.Contains(tagErr.Error(), "NoSuchTagSet") {
			log.Errorf("Tag error for bucket %s: %s", *bucket.Name, tagErr.Error())
			continue
		}

		essentialTags := common.GetEssentialTags(bucketTags.TagSet, tagName)

		taggedS3Buckets = append(taggedS3Buckets, common.CloudProviderResource{
			Identifier:   *bucket.Name,
			Description:  "S3 bucket: " + *bucket.Name,
			CreationDate: essentialTags.CreationDate,
			TTL:          essentialTags.TTL,
			Tag:          essentialTags.Tag,
			IsProtected:  essentialTags.IsProtected,
		})
	}

	return taggedS3Buckets, nil
}

func deleteS3Objects(s3session s3.S3, bucket string, objects []*s3.ObjectIdentifier) error {
	_, err := s3session.DeleteObjects(
		&s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &s3.Delete{
				Objects: objects,
				Quiet:   aws.Bool(false),
			},
		})

	if err != nil {
		return err
	}

	return nil
}

func deleteS3ObjectsVersions(s3session s3.S3, bucket string) error {
	// list all objects
	result, err := s3session.ListObjectVersions(
		&s3.ListObjectVersionsInput{
			Bucket: aws.String(bucket),
		})

	if err != nil {
		return err
	}

	// delete all objects
	var objectsIdentifiers []*s3.ObjectIdentifier
	counter := 0
	for _, version := range result.Versions {
		if counter >= 1000 {
			_ = deleteS3Objects(s3session, bucket, objectsIdentifiers)
			objectsIdentifiers = []*s3.ObjectIdentifier{}
			counter = 0
		}

		objectsIdentifiers = append(objectsIdentifiers,
			&s3.ObjectIdentifier{
				Key:       version.Key,
				VersionId: version.VersionId,
			},
		)

		counter++
	}
	_ = deleteS3Objects(s3session, bucket, objectsIdentifiers)

	// delete all Markers
	objectsIdentifiers = []*s3.ObjectIdentifier{}
	counter = 0
	for _, version := range result.DeleteMarkers {
		if counter >= 1000 {
			_ = deleteS3Objects(s3session, bucket, objectsIdentifiers)
			objectsIdentifiers = []*s3.ObjectIdentifier{}
			counter = 0
		}

		objectsIdentifiers = append(objectsIdentifiers,
			&s3.ObjectIdentifier{
				Key:       version.Key,
				VersionId: version.VersionId,
			},
		)

		counter++
	}
	_ = deleteS3Objects(s3session, bucket, objectsIdentifiers)

	return nil
}

func deleteAllS3Objects(s3session s3.S3, bucket string) error {
	// list all objects
	result, err := s3session.ListObjectsV2(
		&s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
		})

	if err != nil {
		return err
	}

	// delete all objects
	var objectsIdentifiers []*s3.ObjectIdentifier
	counter := 0
	for _, object := range result.Contents {
		if counter >= 1000 {
			_ = deleteS3Objects(s3session, bucket, objectsIdentifiers)
			objectsIdentifiers = []*s3.ObjectIdentifier{}
			counter = 0
		}

		objectsIdentifiers = append(objectsIdentifiers,
			&s3.ObjectIdentifier{
				Key: object.Key,
			},
		)

		counter++
	}

	_ = deleteS3Objects(s3session, bucket, objectsIdentifiers)

	return nil
}

func deleteS3Buckets(s3session s3.S3, bucket string) error {
	log.Infof("Deleting bucket %s in %s", bucket, *s3session.Config.Region)

	// delete objects versions
	err := deleteS3ObjectsVersions(s3session, bucket)
	if err != nil {
		log.Errorf("Error while deleting object version file: %v", err)
		return err
	}

	// delete objects
	err = deleteAllS3Objects(s3session, bucket)
	if err != nil {
		log.Errorf("Error while deleting object file: %v", err)
		return err
	}

	// delete bucket
	_, err = s3session.DeleteBucket(
		&s3.DeleteBucketInput{
			Bucket: &bucket,
		})
	if err != nil {
		return err
	}

	return nil
}

func DeleteExpiredBuckets(sessions AWSSessions, options AwsOptions) {
	buckets, err := listTaggedBuckets(*sessions.S3, options.TagName)
	region := sessions.S3.Config.Region
	if err != nil {
		log.Errorf("can't list S3 buckets: %s\n", err)
		return
	}
	var expiredBuckets []common.CloudProviderResource
	for _, bucket := range buckets {
		if bucket.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredBuckets = append(expiredBuckets, bucket)
		}
	}

	s := fmt.Sprintf("There is no expired S3 bucket to delete in %s.", *region)
	if len(expiredBuckets) == 1 {
		s = fmt.Sprintf("There is 1 expired S3 bucket to delete in %s.", *region)
	}
	if len(expiredBuckets) > 1 {
		s = fmt.Sprintf("There are %d expired S3 buckets to delete.", len(expiredBuckets))
	}

	log.Debug(s)

	if options.DryRun || len(expiredBuckets) == 0 {
		return
	}

	log.Debug("Starting expired S3 buckets deletion.")

	for _, bucket := range expiredBuckets {
		deletionErr := deleteS3Buckets(*sessions.S3, bucket.Identifier)
		if deletionErr != nil {
			log.Errorf("Deletion S3 Bucket %s/%s error: %s",
				bucket.Identifier, *region, err)
		}
	}
}
