package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type KeyPair struct {
	common.CloudProviderResource
	KeyName string
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
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *key.KeyPairId,
				Description:  "EC2 Key Pair: " + *key.KeyPairId,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			KeyName: *key.KeyName,
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
		if key.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	count, start := common.ElemToDeleteFormattedInfos("expired ELB load balancer", len(expiredKeys), *region)

	log.Info(count)

	if options.DryRun || len(expiredKeys) == 0 {
		return
	}

	log.Info(start)

	for _, key := range expiredKeys {
		deletionErr := deleteKeyPair(sessions.EC2, key.Identifier)
		if deletionErr != nil {
			log.Errorf("Deletion EC2 key pair error %s/%s: %s", key.KeyName, *region, deletionErr)
		} else {
			log.Debugf("Key Pair %s in %s deleted.", key.KeyName, *region)
		}
	}
}
