package utils

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
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

func GetRDSTimeInfos(tags []*rds.Tag) (time.Time, int64) {
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
	if ttl == 0  || creationTime == time.Date(0001, 01, 01, 00, 00, 00, 0000, time.UTC){
		return false
	}
	return time.Now().After(expirationTime)
}

func AddCreationDateTag(svc interface{}, idsToTag []*string, creationDate time.Time, ttl int64) error {
	if idsToTag != nil {

		ec2Session, isOk := svc.(ec2.EC2)
		if isOk {
			return  ec2CreationDateTag(ec2Session, idsToTag, creationDate, ttl)
		}

		rdsSession, isOk := svc.(rds.RDS)
		if isOk {
			return  rdsCreationDateTag(rdsSession, idsToTag, creationDate, ttl)
		}
	}

	return nil
}

func ec2CreationDateTag(ec2Session ec2.EC2, idsToTag []*string, creationDate time.Time, ttl int64) error {
	slicedArray := getSlicedArray(idsToTag, 20)

	for _, slice := range slicedArray {
			_, err := ec2Session.CreateTags(
				&ec2.CreateTagsInput{
					Resources: 	slice,
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
				return fmt.Errorf("Can't add tags to %p in region %s: %s", slice, *ec2Session.Config.Region, err.Error())
			}
		}

	return nil
}

func rdsCreationDateTag(rdsSession rds.RDS, idsToTag []*string, creationDate time.Time, ttl int64) error {
	for _, id := range idsToTag {
		_, err := 	rdsSession.AddTagsToResource(
			&rds.AddTagsToResourceInput{
				ResourceName: aws.String(*id),
				Tags: []*rds.Tag{
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
			return fmt.Errorf("Can't add tags to %s in region %s: %s", *id, *rdsSession.Config.Region, err.Error())
		}
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

func getSlicedArray(arrayToSlice []*string, sliceRange int) [][]*string {
	var slicedArray [][]*string
	slicesCount := len(arrayToSlice) / sliceRange + 1

	if len(arrayToSlice) < sliceRange {
		slicedArray = append(slicedArray, arrayToSlice)
	} else {
		for i := 0; i < slicesCount; i++ {
			if (i+1) * sliceRange > len(arrayToSlice) {
				slicedArray = append(slicedArray, arrayToSlice[i*sliceRange:len(arrayToSlice)-1])
			} else {
				slicedArray = append(slicedArray, arrayToSlice[i*sliceRange:(i+1)*sliceRange])
			}
		}
	}


	return slicedArray
}
