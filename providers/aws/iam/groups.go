package iam

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
	"strconv"
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

func DeleteGroups(iamSession *iam.IAM, dryRun bool) error{
	groups := getGroups(iamSession)
	log.Info("There is " + strconv.FormatInt(int64(len(groups)), 10) + " expired roles to delete.")

	if dryRun {
		return nil
	}

	for _, group := range groups {
		_, err := iamSession.DeleteGroup(
			&iam.DeleteGroupInput{
				GroupName: aws.String(*group.GroupName),
				})

		if err != nil {
			return err
		}
	}

	return nil
}

