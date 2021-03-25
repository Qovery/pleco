package utils

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"strconv"
	"time"
)


func GetTimeInfos(tags []*ec2.Tag) (time.Time, int64) {
	var creationDate = time.Time{}
	var ttl int64

	for i := range tags {
		if *tags[i].Key == "creationDate" {
			creationTime, _ := strconv.ParseInt(*tags[i].Value, 10, 64)
			creationDate = time.Unix(creationTime,0)
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

func AddCreationDateTag (ec2Session ec2.EC2, idsToTag []*string, creationDate time.Time, ttl int64) error {
	_, err := ec2Session.CreateTags(
		&ec2.CreateTagsInput{
			Resources: 	idsToTag,
			Tags: []*ec2.Tag{
				{
					Key: aws.String("creationDate"),
					Value: aws.String(creationDate.String()),
				},
				{
					Key: aws.String("ttl"),
					Value: aws.String(strconv.FormatInt(ttl,10)),
				},
			},
		})

	if err != nil {
		return err
	}

	return nil
}

func ElemToDeleteFormattedInfos(elemName string, arraySize int, region string) (string,string) {
	count := fmt.Sprintf("There is no %s to delete in region %s.", elemName,region)
	if arraySize == 1 {
		count = fmt.Sprintf("There is no %s to delete in region %s.", elemName,region)
	}
	if arraySize > 1 {
		count = fmt.Sprintf("There are %d %ss to delete in region %s.", arraySize, elemName,region)
	}

	start := fmt.Sprintf("Starting %s deletion for region %s.", elemName,region)


	return count, start
}


func AwsStringChecker(elem interface{ String() string }) string {
	if elem != nil {
		return elem.String()
	} else {
		return ""
	}
}