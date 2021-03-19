package ec2

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

type KeyPair struct {
	KeyName string
	KeyId string
	CreationDate time.Time
	Tag string
	ttl int64
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
		newKey := KeyPair{
			KeyName: *key.KeyName,
			KeyId: *key.KeyPairId,
		}

		for _, tag := range key.Tags {
			if strings.EqualFold(*tag.Key, tagName){
				newKey.Tag = *tag.Value
			}
			if strings.EqualFold(*tag.Key, "ttl"){
				ttl, _ := strconv.Atoi(*tag.Value)
				newKey.ttl = int64(ttl)
			}
		}

		if newKey.ttl != 0 {
			keys = append(keys, newKey)
		}
	}

	return keys
}

func deleteKey (ec2session *ec2.EC2, keyId string) error {
	_, err := ec2session.DeleteKeyPair(
		&ec2.DeleteKeyPairInput{
			KeyPairId: aws.String(keyId),
		})

	return err
}

func DeleteExpiredKeys (ec2session *ec2.EC2, tagName string, dryRun bool) error {
	keys := getSshKeys(ec2session, tagName)
	var expiredKeysCount int64

	for _, key := range keys {
		if utils.CheckIfExpired(key.CreationDate, key.ttl) {
			expiredKeysCount++

			if !dryRun {
				err := deleteKey(ec2session, key.KeyId)
				if err != nil {
					return err
				}
			}
		}
	}

	log.Infof("There is %d expired key(s) to delete", expiredKeysCount)

	return nil
}
