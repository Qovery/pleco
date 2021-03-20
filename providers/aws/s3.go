package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type s3Bucket struct {
	Name       string
	CreateTime time.Time
	TTL        int64
}

func listTaggedBuckets(s3Session *s3.S3, tagName string) ([]s3Bucket, error) {
	var taggedS3Buckets []s3Bucket
	currentRegion := s3Session.Config.Region

	log.Debugf("Listing all S3 buckets in %s", *s3Session.Config.Region)
	input := &s3.ListBucketsInput{}

	result, err := s3Session.ListBuckets(input)
	if err != nil {
		return nil, err
	}

	if len(result.Buckets) == 0 {
		log.Debug("No S3 were found")
		return nil, nil
	}

	for _, bucket := range result.Buckets {
		bucketLocationinput := &s3.GetBucketLocationInput{
			Bucket: aws.String(*bucket.Name),
		}
		location, err := s3Session.GetBucketLocation(bucketLocationinput)
		if err != nil {
			continue
		}

		if *location.LocationConstraint != *currentRegion {
			continue
		}

		input := &s3.GetBucketTaggingInput{
			Bucket: aws.String(*bucket.Name),
		}

		bucketTags, err := s3Session.GetBucketTagging(input)
		if err != nil {
			continue
		}

		for _, tag := range bucketTags.TagSet {
			if *tag.Key == tagName {
				if *tag.Key == "" {
					log.Warnf("Tag %s was empty and it wasn't expected, skipping", *tag.Key)
					continue
				}

				ttl, err := strconv.Atoi(*tag.Value)
				if err != nil {
					log.Errorf("Error while trying to convert tag value (%s) to integer on S3 Bucket %s in %v",
						*tag.Value, *bucket.Name, s3Session.Config.Region)
					continue
				}

				taggedS3Buckets = append(taggedS3Buckets, s3Bucket{
					Name:       *bucket.Name,
					CreateTime: *bucket.CreationDate,
					TTL:        int64(ttl),
				})
			}
		}

	}

	log.Debugf("Found %d S3 buckets in %s with %s tag", len(taggedS3Buckets), *s3Session.Config.Region, tagName)

	return taggedS3Buckets, nil
}

func deleteS3Objects(s3session *s3.S3, bucket string, objects []*s3.ObjectIdentifier) error {
	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3.Delete{
			Objects: objects,
			Quiet:   aws.Bool(false),
		},
	}

	_, err := s3session.DeleteObjects(input)
	if err != nil {
		return err
	}

	return nil
}

func deleteS3ObjectsVersions(s3session *s3.S3, bucket string) error {
	// list all objects
	input := &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
	}
	result, err := s3session.ListObjectVersions(input)
	if err != nil {
		return err
	}

	// delete all objects
	objectsIdentifiers := []*s3.ObjectIdentifier{}
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

func deleteAllS3Objects(s3session *s3.S3, bucket string) error {
	// list all objects
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}
	result, err := s3session.ListObjectsV2(input)
	if err != nil {
		return err
	}

	// delete all objects
	objectsIdentifiers := []*s3.ObjectIdentifier{}
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

func deleteS3Buckets(s3session *s3.S3, bucket string, dryRun bool) error {
	if dryRun {
		return nil
	}

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

func DeleteExpiredBuckets(sessions *AWSSessions, options *AwsOption) error {
	buckets, err := listTaggedBuckets(sessions.S3, options.TagName)
	if err != nil {
		return fmt.Errorf("can't list S3 buckets: %s\n", err)
	}

	for _, bucket := range buckets {
		if CheckIfExpired(bucket.CreateTime, bucket.TTL) {
			err := deleteS3Buckets(sessions.S3, bucket.Name, options.DryRun)
			if err != nil {
				log.Errorf("Deletion S3 Bucket %s/%s error: %s",
					bucket.Name, *sessions.S3.Config.Region, err)
				continue
			}
		} else {
			log.Debugf("S3 bucket %s in %s, has not yet expired",
				bucket.Name, *sessions.S3.Config.Region)
		}
	}

	return nil
}
