package aws

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type RouteTable struct {
	Id           string
	CreationDate time.Time
	ttl          int64
	Associations []*ec2.RouteTableAssociation
	IsProtected  bool
}

func getRouteTablesByVpcId(ec2Session *ec2.EC2, vpcId string) []*ec2.RouteTable {
	input := &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	routeTables, err := ec2Session.DescribeRouteTables(input)
	if err != nil {
		log.Error(err)
	}

	return routeTables.RouteTables
}

func GetRouteTablesIdsByVpcId(ec2Session *ec2.EC2, vpcId string, tagName string) []RouteTable {
	var routeTablesStruct []RouteTable

	routeTables := getRouteTablesByVpcId(ec2Session, vpcId)

	for _, routeTable := range routeTables {
		essentialTags := common.GetEssentialTags(routeTable.Tags, tagName)

		var routeTableStruct = RouteTable{
			Id:           *routeTable.RouteTableId,
			CreationDate: essentialTags.CreationDate,
			ttl:          essentialTags.TTL,
			Associations: routeTable.Associations,
			IsProtected:  essentialTags.IsProtected,
		}
		routeTablesStruct = append(routeTablesStruct, routeTableStruct)
	}

	return routeTablesStruct
}

func DeleteRouteTablesByIds(ec2Session *ec2.EC2, routeTables []RouteTable) {
	for _, routeTable := range routeTables {
		if !isMainRouteTable(routeTable) && !routeTable.IsProtected {
			_, err := ec2Session.DeleteRouteTable(
				&ec2.DeleteRouteTableInput{
					RouteTableId: aws.String(routeTable.Id),
				},
			)

			if err != nil {
				log.Error(err)
			} else {
				log.Debugf("Route table %s in %s deleted.", routeTable.Id, *ec2Session.Config.Region)
			}
		}
	}
}

func isMainRouteTable(routeTable RouteTable) bool {
	for _, association := range routeTable.Associations {
		if *association.Main && routeTable.Id == *association.RouteTableId {
			return true
		}
	}

	return false
}
