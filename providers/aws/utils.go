package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"strconv"
	"time"
)

func GetTimeInfos(tags []*ec2.Tag) (time.Time, int64) {
	var creationDate = time.Now()
	var ttl int64

	for i := range tags {
		if *tags[i].Key == "creationDate" {
			creationTime, _ := strconv.ParseInt(*tags[i].Value, 10, 64)
			creationDate = time.Unix(creationTime, 0)
		}
		if *tags[i].Key == "ttl" {
			result, _ := strconv.ParseInt(*tags[i].Value, 10, 64)
			ttl = result
		}
	}

	return creationDate, ttl
}

func CheckIfExpired(creationTime time.Time, ttl int64) bool {
	expirationTime := creationTime.Add(time.Duration(ttl) * time.Second)
	return time.Now().After(expirationTime)
}

func AddCreationDateTag(ec2Session ec2.EC2, idsToTag []*string, creationDate time.Time, ttl int64) error {
	_, err := ec2Session.CreateTags(
		&ec2.CreateTagsInput{
			Resources: idsToTag,
			Tags: []*ec2.Tag{
				{
					Key:   aws.String("creationDate"),
					Value: aws.String(creationDate.String()),
				},
				{
					Key:   aws.String("ttl"),
					Value: aws.String(strconv.FormatInt(ttl, 10)),
				},
			},
		})

	if err != nil {
		return err
	}

	return nil
}
