package common

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ecr"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sfn"
	log "github.com/sirupsen/logrus"
)

type Tag struct {
	_     struct{} `type:"structure"`
	Key   *string  `type:"string"`
	Value *string  `type:"string"`
}

type MyTag struct {
	_     struct{} `type:"structure"`
	Key   string   `type:"string"`
	Value string   `type:"string"`
}

type EssentialTags struct {
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
	ClusterId    string
	Tag          string
}

type CloudProviderResource struct {
	Identifier   string
	Description  string
	CreationDate time.Time
	TTL          int64
	Tag          string
	IsProtected  bool
}

func (resource *CloudProviderResource) IsResourceExpired(commandLineTagValue string, disableTTLCheck bool) bool {
	if resource.IsProtected {
		return false
	}
	isDestroyingCommand := strings.TrimSpace(commandLineTagValue) != ""
	if isDestroyingCommand {
		return strings.EqualFold(resource.Tag, commandLineTagValue)
	} else {
		return CheckIfExpired(resource.CreationDate, resource.TTL, resource.Description, disableTTLCheck)
	}
}

func GetEssentialTags(tagsInput interface{}, tagName string) EssentialTags {
	var tags []MyTag

	switch typedTags := tagsInput.(type) {
	case []*rds.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case []*ec2.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case []*iam.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case []*kms.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.TagKey, Value: *elem.TagValue})
		}
	case []*s3.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case []*elbv2.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case []*elasticache.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case []*sfn.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case []*cloudformation.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case []*ecr.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case []*eventbridge.Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case []*Tag:
		for _, elem := range typedTags {
			tags = append(tags, MyTag{Key: *elem.Key, Value: *elem.Value})
		}
	case map[string]*string:
		for key, value := range typedTags {
			tags = append(tags, MyTag{Key: key, Value: *value})
		}
	case []string:
		for _, value := range typedTags {
			if strings.Contains(value, "=") {
				val := strings.SplitN(value, "=", 2)
				tags = append(tags, MyTag{Key: val[0], Value: val[1]})
			}

			if strings.Contains(value, ":") {
				val := strings.SplitN(value, ":", 2)
				tags = append(tags, MyTag{Key: val[0], Value: val[1]})
			}
		}
	default:
		log.Debugf("Can't parse tags %s.", tagsInput)
	}

	essentialTags := EssentialTags{}
	essentialTags.TTL = -1
	for i := range tags {
		switch tags[i].Key {
		case "creationDate", "CreationDate":
			essentialTags.CreationDate = stringDateToTimeDate(tags[i].Value)
		case "ttl", "Ttl", "TTL":
			result, _ := strconv.ParseInt(tags[i].Value, 10, 64)
			essentialTags.TTL = result
		case "do_not_delete":
			result, _ := strconv.ParseBool(tags[i].Value)
			essentialTags.IsProtected = result
		case "ClusterId":
			essentialTags.ClusterId = tags[i].Value
		default:
			continue
		}
	}
	// if 'tagName' value is present in above switch, then he won't be filled
	for i := range tags {
		if strings.EqualFold(tags[i].Key, tagName) {
			essentialTags.Tag = tags[i].Value
		}
	}

	return essentialTags
}

func CheckIfExpired(creationTime time.Time, ttl int64, resourceNameDescription string, disableTTLCheck bool) bool {
	if ttl == -1 && disableTTLCheck {
		return time.Now().UTC().After(creationTime.Add(4 * time.Hour))
	}

	if ttl == 0 {
		log.Debugf("TTL tag is set to 0. Skipping %s ...", resourceNameDescription)
		return false
	}

	if ttl == -1 {
		log.Warnf("TTL tag is missing. Can't check if resource %s is expired.", resourceNameDescription)
		return false
	}

	expirationTime := creationTime.UTC().Add(time.Duration(ttl) * time.Second)

	if creationTime.Year() < 1972 {
		log.Warnf("Creation date tag is missing. Can't check if resource %s is expired.", resourceNameDescription)
		return false
	}

	return time.Now().UTC().After(expirationTime)
}

func ElemToDeleteFormattedInfos(elemName string, arraySize int, region string, isZone ...bool) (string, string) {
	regionString := fmt.Sprintf(" in region %s", region)
	if region == "" {
		regionString = ""
	}

	if isZone != nil && isZone[0] {
		regionString = fmt.Sprintf(" in zone %s", region)
	}

	count := fmt.Sprintf("There is no %s to delete%s.", elemName, regionString)
	if arraySize == 1 {
		count = fmt.Sprintf("There is 1 %s to delete%s.", elemName, regionString)
	}
	if arraySize > 1 {
		count = fmt.Sprintf("There are %d %ss to delete%s.", arraySize, elemName, regionString)
	}

	start := fmt.Sprintf("Starting %s deletion%s.", elemName, regionString)

	return count, start
}

func IsAssociatedToLivingCluster(tagsInput interface{}, svc *eks.EKS) bool {
	result, clusterErr := svc.ListClusters(&eks.ListClustersInput{})
	if clusterErr != nil {
		log.Error("Can't list cluster for ELB association check")
		return false
	}

	switch typedTags := tagsInput.(type) {
	case []*elbv2.Tag:
		for _, cluster := range result.Clusters {
			for _, tag := range typedTags {
				// ALB controller key contains '/cluster' and cluster name is the value
				if strings.Contains(*tag.Key, "/cluster") && *tag.Value == *cluster {
					return true
				}
				// while kubernetes built-in NLB tag is'/cluster/' with cluster name
				if strings.Contains(*tag.Key, "/cluster") && strings.Contains(*tag.Key, *cluster) {
					return true
				}
			}
		}
	case []*ec2.Tag:
		for _, cluster := range result.Clusters {
			for _, tag := range typedTags {
				if strings.Contains(*tag.Key, "/cluster/") && strings.Contains(*tag.Key, *cluster) {
					return true
				}
			}
		}
	default:
		log.Debugf("Can't parse tags %s.", tagsInput)
	}

	return false
}

func stringDateToTimeDate(date string) time.Time {
	year, _ := strconv.ParseInt(date[0:4], 10, 32)
	month, _ := strconv.ParseInt(date[5:7], 10, 32)
	day, _ := strconv.ParseInt(date[8:10], 10, 32)
	if len(date) <= 10 {
		return time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, time.UTC)
	}

	hour, _ := strconv.ParseInt(date[11:13], 10, 32)
	minutes, _ := strconv.ParseInt(date[14:16], 10, 32)
	seconds, _ := strconv.ParseInt(date[17:18], 10, 32)

	return time.Date(int(year), time.Month(month), int(day), int(hour), int(minutes), int(seconds), 0, time.UTC)
}

func CheckSnapshot(snap *rds.DBSnapshot) bool {
	return strings.Contains(*snap.Status, "available") && !strings.Contains(*snap.DBSnapshotIdentifier, "default:")
}

func CheckClusterSnapshot(snap *rds.DBClusterSnapshot) bool {
	return strings.Contains(*snap.Status, "available") && !strings.Contains(*snap.DBClusterSnapshotIdentifier, "default:")
}

func CheckElasticacheSnapshot(snap *elasticache.Snapshot) bool {
	return strings.Contains(*snap.SnapshotStatus, "available") && !strings.Contains(*snap.SnapshotName, "default:")
}
