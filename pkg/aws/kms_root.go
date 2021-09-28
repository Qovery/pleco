package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/kms"
	log "github.com/sirupsen/logrus"
	"time"
)

type CompleteKey struct {
	KeyId        string
	TTL          int64
	Tag          string
	Status       string
	CreationDate time.Time
	IsProtected  bool
}

func getKeys(svc kms.KMS) []*kms.KeyListEntry {
	input := &kms.ListKeysInput{
		Limit: aws.Int64(1000),
	}

	keys, err := svc.ListKeys(input)
	handleKMSError(err)

	return keys.Keys
}

func getCompleteKey(svc kms.KMS, keyId *string, tagName string) CompleteKey {
	tags := getKeyTags(svc, keyId)
	metaData := getKeyMetadata(svc, keyId)

	essentialTags := common.GetEssentialTags(tags, tagName)

	return CompleteKey{
		KeyId:        *keyId,
		Status:       *metaData.KeyMetadata.KeyState,
		CreationDate: essentialTags.CreationDate,
		TTL:          essentialTags.TTL,
		IsProtected:  essentialTags.IsProtected,
	}
}

func deleteKey(svc kms.KMS, keyId string) (*kms.ScheduleKeyDeletionOutput, error) {
	input := &kms.ScheduleKeyDeletionInput{
		KeyId:               aws.String(keyId),
		PendingWindowInDays: aws.Int64(7),
	}

	result, err := svc.ScheduleKeyDeletion(input)
	handleKMSError(err)

	return result, err
}

func getKeyTags(svc kms.KMS, keyId *string) []*kms.Tag {
	input := &kms.ListResourceTagsInput{
		KeyId: aws.String(*keyId),
	}

	tags, err := svc.ListResourceTags(input)
	handleKMSError(err)

	return tags.Tags
}

func getKeyMetadata(svc kms.KMS, keyId *string) *kms.DescribeKeyOutput {
	input := &kms.DescribeKeyInput{KeyId: keyId}

	data, err := svc.DescribeKey(input)
	handleKMSError(err)

	return data
}

func handleKMSError(error error) {
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

func DeleteExpiredKeys(sessions *AWSSessions, options *AwsOptions) {
	keys := getKeys(*sessions.KMS)
	region := sessions.KMS.Config.Region
	var expiredKeys []CompleteKey
	for _, key := range keys {
		completeKey := getCompleteKey(*sessions.KMS, key.KeyId, options.TagName)

		if completeKey.Status != "PendingDeletion" && completeKey.Status != "Disabled" &&
			common.CheckIfExpired(completeKey.CreationDate, completeKey.TTL, "kms key: "+completeKey.KeyId) && !completeKey.IsProtected {
			if completeKey.Tag == options.TagName || options.TagName == "ttl" {
				expiredKeys = append(expiredKeys, completeKey)
			}
		}
	}

	count, start := common.ElemToDeleteFormattedInfos("expired KMS key", len(expiredKeys), *region)

	log.Debug(count)

	if options.DryRun || len(expiredKeys) == 0 {
		return
	}

	log.Debug(start)

	for _, key := range expiredKeys {
		_, deletionErr := deleteKey(*sessions.KMS, key.KeyId)
		if deletionErr != nil {
			log.Errorf("Deletion KMS key error %s/%s: %s",
				key.KeyId, *region, deletionErr)
		}
	}
}
