package aws

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/kms"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type CompleteKey struct {
	KeyId string
	TTL int64
	Tag string
	Status string
	CreationDate time.Time
}


func getKeys(svc kms.KMS) []*kms.KeyListEntry{
	input := &kms.ListKeysInput{
		Limit: aws.Int64(1000),
	}

	keys, err := svc.ListKeys(input)
	handleKMSError(err)

	return keys.Keys
}

func getCompleteKey(svc kms.KMS, keyId *string, tagName string) CompleteKey {
	var completeKey CompleteKey
	tags := getKeyTags(svc,keyId)
	metaData := getKeyMetadata(svc,keyId)

	completeKey.KeyId = *keyId
	completeKey.Status = *metaData.KeyMetadata.KeyState
	completeKey.CreationDate = *metaData.KeyMetadata.CreationDate

	for i := range tags {
		if *tags[i].TagKey == "ttl" {
			ttl , _ := strconv.ParseInt(*tags[i].TagValue,10,64)
			completeKey.TTL = ttl
		}

		if *tags[i].TagKey == tagName {
			completeKey.Tag = *tags[i].TagValue
		}
	}

	return completeKey
}

func deleteKey(svc kms.KMS,keyId *string) (*kms.ScheduleKeyDeletionOutput,error){
	input := &kms.ScheduleKeyDeletionInput{
		KeyId:               aws.String(*keyId),
		PendingWindowInDays: aws.Int64(7),
	}

	result, err := svc.ScheduleKeyDeletion(input)
	handleKMSError(err)

	return result,err
}

func getKeyTags (svc kms.KMS, keyId *string) []*kms.Tag {
	input := &kms.ListResourceTagsInput{
		KeyId: aws.String(*keyId),
	}

	tags, err := svc.ListResourceTags(input)
	handleKMSError(err)

	return tags.Tags
}

func getKeyMetadata (svc kms.KMS,keyId *string) *kms.DescribeKeyOutput{
	input := &kms.DescribeKeyInput{KeyId: keyId}

	data, err := svc.DescribeKey(input)
	handleKMSError(err)

	return data
}

func handleKMSError (error error) {
	if error != nil {
		if aerr, ok := error.(awserr.Error); ok {
			switch aerr.Code() {
			case kms.ErrCodeNotFoundException:
				log.Error(kms.ErrCodeNotFoundException, aerr.Error())
			case kms.ErrCodeInvalidArnException:
				log.Error(kms.ErrCodeInvalidArnException, aerr.Error())
			case kms.ErrCodeDependencyTimeoutException:
				log.Error(kms.ErrCodeDependencyTimeoutException, aerr.Error())
			case kms.ErrCodeInternalException:
				log.Error(kms.ErrCodeInternalException, aerr.Error())
			case kms.ErrCodeInvalidStateException:
				log.Error(kms.ErrCodeInvalidStateException, aerr.Error())
			default:
				log.Error(aerr.Error())
			}
		} else {
			log.Error(error.Error())
		}

	}
}

func deleteExpiredKeys(svc kms.KMS, tagName string, dryRun bool) error{
	keys := getKeys(svc)
	var numberOfKeysToDelete int64

	for _, key := range keys {
		completeKey := getCompleteKey(svc, key.KeyId, tagName)

		if completeKey.Status != "PendingDeletion" &&
			completeKey.TTL != 0 &&
			utils.CheckIfExpired(completeKey.CreationDate,  completeKey.TTL) {
			if completeKey.Tag == tagName || tagName == "ttl"{
				if !dryRun {
					_, err := deleteKey(svc, key.KeyId)
					if err != nil {
						return err
					}
				}

				numberOfKeysToDelete += 1
			}
		}
	}

	log.Info("There is ", numberOfKeysToDelete, " expired keys to delete")

	return nil
}