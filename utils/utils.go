package utils

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type Tag struct {
	_     struct{} `type:"structure"`
	Key   *string  `type:"string"`
	Value *string  `type:"string"`
}

type MyTag struct {
	_     struct{} `type:"structure"`
	Key   string  `type:"string"`
	Value string  `type:"string"`
}

func GetEssentialTags(tagsInput interface{}, tagName string) (time.Time, int64, bool, string, string) {
	var creationDate = time.Time{}
	var ttl int64
	var isProtected bool
	var clusterId string
	var tag string
	var tags []MyTag

	switch tagsInput.(type) {
		case []*rds.Tag:
			m := tagsInput.([]*rds.Tag)
			for _, elem := range m {
				tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
			}
		case []*ec2.Tag:
			m := tagsInput.([]*ec2.Tag)
			for _, elem := range m {
				tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
			}
		case []*iam.Tag:
			m := tagsInput.([]*iam.Tag)
			for _, elem := range m {
				tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
			}
		case []*kms.Tag:
			m := tagsInput.([]*kms.Tag)
			for _, elem := range m {
				tags = append(tags, MyTag{Key: *elem.TagKey, Value: *elem.TagValue})
			}
		case []*s3.Tag:
			m := tagsInput.([]*s3.Tag)
			for _, elem := range m {
				tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
			}
		case []*elbv2.Tag:
			m := tagsInput.([]*elbv2.Tag)
			for _, elem := range m {
				tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
			}
		case []*elasticache.Tag:
			m := tagsInput.([]*elasticache.Tag)
			for _, elem := range m {
				tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
			}
		case []*Tag:
			m := tagsInput.([]*Tag)
			for _, elem := range m {
				tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
			}
		case map[string]*string:
			m := tagsInput.(map[string]*string)
			for key, value := range m {
				tags = append(tags, MyTag{Key: key, Value: *value})
			}
		default:
			log.Debugf("Can't parse tags %s.", tagsInput)
	}

	for i := range tags {
		switch tags[i].Key {
			case "creationDate":
				creationDate = stringDateToTimeDate(tags[i].Value)
			case "ttl":
				result, _ := strconv.ParseInt(tags[i].Value, 10, 64)
				ttl = result
			case "do_not_delete":
				result, _ := strconv.ParseBool(tags[i].Value)
				isProtected = result
			case "ClusterId":
				clusterId = tags[i].Value
			case tagName:
				tag = tags[i].Value
			default:
				continue
			}
	}

	return creationDate, ttl, isProtected, clusterId, tag
}

func CheckIfExpired(creationTime time.Time, ttl int64) bool {
	expirationTime := creationTime.Add(time.Duration(ttl) * time.Second)
	if ttl == 0  || creationTime.Year() < 1972 {
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
		count = fmt.Sprintf("There is 1 %s to delete in region %s.", elemName,region)
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

	if len(arrayToSlice) <= sliceRange {
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

func stringDateToTimeDate(date string) time.Time {
	year, _ := strconv.ParseInt(date[0:4], 10, 32)
	month, _ := strconv.ParseInt(date[5:7], 10, 32)
	day, _ := strconv.ParseInt(date[8:10], 10, 32)
	hour, _ := strconv.ParseInt(date[11:13], 10, 32)
	minutes, _ := strconv.ParseInt(date[14:16], 10, 32)
	seconds, _ := strconv.ParseInt(date[17:18], 10, 32)

	return time.Date(int(year), time.Month(month), int(day), int(hour), int(minutes), int(seconds), 0, time.Local)
}




