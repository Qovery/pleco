package aws

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type s3Bucket struct {
	common.CloudProviderResource
	ObjectsCount int
}

func listTaggedBuckets(s3Session s3.S3, tagName string) ([]s3Bucket, error) {
	var taggedS3Buckets []s3Bucket
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

		result, objectErr := s3Session.ListObjectVersions(
			&s3.ListObjectVersionsInput{
				Bucket: aws.String(*bucket.Name),
			})
		if objectErr != nil {
			log.Errorf("Listing object error for bucket %s: %s", *bucket.Name, locationErr.Error())
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

		taggedS3Buckets = append(taggedS3Buckets,
			s3Bucket{
				CloudProviderResource: common.CloudProviderResource{
					Identifier:   *bucket.Name,
					Description:  "S3 bucket: " + *bucket.Name,
					CreationDate: bucket.CreationDate.UTC(),
					TTL:          essentialTags.TTL,
					Tag:          essentialTags.Tag,
					IsProtected:  essentialTags.IsProtected,
				},
				ObjectsCount: len(result.Versions),
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

func deleteS3BucketPolicy(s3session s3.S3, bucket string) error {
	log.Infof("Deleting policy for bucket %s in %s", bucket, *s3session.Config.Region)

	// delete bucket policy
	_, err := s3session.DeleteBucketPolicy(
		&s3.DeleteBucketPolicyInput{
			Bucket: &bucket,
		})
	if err != nil {
		return err
	}

	return nil
}

func deleteS3Buckets(s3session s3.S3, bucket string) error {
	log.Infof("Deleting bucket %s in %s", bucket, *s3session.Config.Region)

	// delete bucket policy
	err := deleteS3BucketPolicy(s3session, bucket)
	if err != nil {
		log.Errorf("Error while deleting kucket policy: %v", err)
		return err
	}

	// delete objects versions
	err = deleteS3ObjectsVersions(s3session, bucket)
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
		log.Errorf("Can't list S3 buckets: %s\n", err)
		return
	}
	var expiredBuckets []s3Bucket
	for _, bucket := range buckets {
		// Set to 2 weeks to avoid terraform issue when recreating the bucket after Pleco deleted it, will revert once fixed in terraform/aws provider (QOV-1033)
		if (bucket.ObjectsCount == 0 && time.Now().UTC().After(bucket.CreationDate.Add(14*24*time.Hour))) || bucket.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
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

	log.Info(s)

	if options.DryRun || len(expiredBuckets) == 0 {
		return
	}

	log.Info("Starting expired S3 buckets deletion.")

	for _, bucket := range expiredBuckets {
		deletionErr := deleteS3Buckets(*sessions.S3, bucket.Identifier)
		if deletionErr != nil {
			log.Errorf("Deletion S3 Bucket %s/%s error: %s",
				bucket.Identifier, *region, err)
		} else {
			log.Debugf("S3 bucket %s in %s deleted.", bucket.Identifier, *region)
		}
	}
}
