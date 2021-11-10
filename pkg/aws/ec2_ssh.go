package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"time"
)

type KeyPair struct {
	KeyName      string
	KeyId        string
	CreationDate time.Time
	Tag          string
	ttl          int64
	IsProtected  bool
}

func getSshKeys(ec2session *ec2.EC2, tagName string) []KeyPair {
	result, err := ec2session.DescribeKeyPairs(
		&ec2.DescribeKeyPairsInput{})

	if err != nil {
		log.Error(err)
		return nil
	}

	var keys []KeyPair
	for _, key := range result.KeyPairs {
		essentialTags := common.GetEssentialTags(key.Tags, tagName)
		newKey := KeyPair{
			KeyName:      *key.KeyName,
			KeyId:        *key.KeyPairId,
			CreationDate: essentialTags.CreationDate,
			ttl:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
		}

		keys = append(keys, newKey)
	}

	return keys
}

func deleteKeyPair(ec2session *ec2.EC2, keyId string) error {
	_, err := ec2session.DeleteKeyPair(
		&ec2.DeleteKeyPairInput{
			KeyPairId: aws.String(keyId),
		})

	return err
}

func DeleteExpiredKeyPairs(sessions AWSSessions, options AwsOptions) {
	keys := getSshKeys(sessions.EC2, options.TagName)
	region := sessions.EC2.Config.Region
	var expiredKeys []KeyPair
	for _, key := range keys {
		if common.CheckIfExpired(key.CreationDate, key.ttl, "ec2 key pair: "+key.KeyId) && !key.IsProtected {
			expiredKeys = append(expiredKeys, key)
		}
	}

	count, start := common.ElemToDeleteFormattedInfos("expired ELB load balancer", len(expiredKeys), *region)

	log.Debug(count)

	if options.DryRun || len(expiredKeys) == 0 {
		return
	}

	log.Debug(start)

	for _, key := range expiredKeys {
		deletionErr := deleteKeyPair(sessions.EC2, key.KeyId)
		if deletionErr != nil {
			log.Errorf("Deletion EC2 key pair error %s/%s: %s",
				key.KeyName, *region, deletionErr)
		}
	}
}
