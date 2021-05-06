package ec2

import (
	"github.com/Qovery/pleco/utils"
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

func getSshKeys (ec2session *ec2.EC2, tagName string) []KeyPair {
	result, err := ec2session.DescribeKeyPairs(
		&ec2.DescribeKeyPairsInput{

		})

	if err !=nil {
		log.Error(err)
		return nil
	}

	var keys []KeyPair
	for _, key := range result.KeyPairs {
		creationTime, ttl, isProtected, _, _ := utils.GetEssentialTags(key.Tags, tagName)
		newKey := KeyPair{
			KeyName: *key.KeyName,
			KeyId: *key.KeyPairId,
			CreationDate: creationTime,
			ttl: ttl,
			IsProtected: isProtected,
		}

		keys = append(keys, newKey)
	}

	return keys
}

func TagSshKeys(ec2session ec2.EC2, clusterName string, clusterCreationTime time.Time, clusterTtl int64) error {
	keys := getSshKeys(&ec2session, "ttl")
	var keysIds []*string
	for _, key := range keys {
		if key.KeyName == clusterName {
			keysIds = append(keysIds, &key.KeyId)
		}
	}

	return utils.AddCreationDateTag(ec2session, keysIds, clusterCreationTime, clusterTtl)
}

func deleteKey (ec2session *ec2.EC2, keyId string) error {
	_, err := ec2session.DeleteKeyPair(
		&ec2.DeleteKeyPairInput{
			KeyPairId: aws.String(keyId),
		})

	return err
}

func DeleteExpiredKeys (ec2session *ec2.EC2, tagName string, dryRun bool) {
	keys := getSshKeys(ec2session, tagName)
	region := ec2session.Config.Region
	var expiredKeys []KeyPair
	for _, key := range keys {
		if utils.CheckIfExpired(key.CreationDate, key.ttl) && !key.IsProtected {
			expiredKeys = append(expiredKeys, key)
		}
	}

	count, start:= utils.ElemToDeleteFormattedInfos("expired ELB load balancer", len(expiredKeys), *region)

	log.Debug(count)

	if dryRun || len(expiredKeys) == 0 {
		return
	}

	log.Debug(start)

	for _, key := range expiredKeys {
		deletionErr := deleteKey(ec2session, key.KeyId)
		if deletionErr != nil {
			log.Errorf("Deletion EC2 key pair error %s/%s: %s",
				key.KeyName, *region, deletionErr)
		}
	}
}
