package aws

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type RouteTable struct {
	Id           string
	CreationDate time.Time
	ttl          int64
	Associations []*ec2.RouteTableAssociation
	IsProtected  bool
}

func getRouteTablesByVpcId (ec2Session ec2.EC2, vpcId string) []*ec2.RouteTable {
	input := &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("vpc-id"),
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

func SetRouteTablesIdsByVpcId (ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup, tagName string)  {
	defer waitGroup.Done()
	var routeTablesStruct []RouteTable

	routeTables := getRouteTablesByVpcId(ec2Session, *vpc.VpcId)

	for _, routeTable := range routeTables {
		creationDate, ttl, isProtected, _, _:= utils.GetEssentialTags(routeTable.Tags, tagName)

		var routeTableStruct = RouteTable{
			Id: 			*routeTable.RouteTableId,
			CreationDate: 	creationDate,
			ttl: 			ttl,
			Associations: 	routeTable.Associations,
			IsProtected:	isProtected,
		}
		routeTablesStruct = append(routeTablesStruct, routeTableStruct)
	}

	vpc.RouteTables = routeTablesStruct
}

func DeleteRouteTablesByIds (ec2Session ec2.EC2, routeTables []RouteTable) {
	for _, routeTable := range routeTables {
		if utils.CheckIfExpired(routeTable.CreationDate, routeTable.ttl, "vpc route table: " + routeTable.Id) && !isMainRouteTable(routeTable) && !routeTable.IsProtected{
			_, err := ec2Session.DeleteRouteTable(
				&ec2.DeleteRouteTableInput{
					RouteTableId: aws.String(routeTable.Id),
				},
			)

			if err != nil {
				log.Error(err)
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
