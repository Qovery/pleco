package aws

import (
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
)

func getGroups(iamSession *iam.IAM) []*iam.Group {
	result, err := iamSession.ListGroups(
		&iam.ListGroupsInput{
			MaxItems: aws.Int64(1000),
		})

	if err != nil {
		log.Error(err)
		return nil
	}

	return result.Groups
}

func DeleteGroups(iamSession *iam.IAM, dryRun bool) {
	groups := getGroups(iamSession)
	log.Info("There is " + strconv.FormatInt(int64(len(groups)), 10) + " expired roles to delete.")

	if dryRun {
		return
	}

	for _, group := range groups {
		_, err := iamSession.DeleteGroup(
			&iam.DeleteGroupInput{
				GroupName: aws.String(*group.GroupName),
			})

		if err != nil {
			log.Errorf("Can't delete group %s : %s", *group.GroupName, err)
		} else {
			log.Debugf("IAM group %s in %s deleted.", group.GroupName, *iamSession.Config.Region)
		}
	}

}
