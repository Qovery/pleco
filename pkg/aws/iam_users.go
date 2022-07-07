package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type User struct {
	common.CloudProviderResource
}

func getUsers(iamSession *iam.IAM, tagName string) []User {
	result, err := iamSession.ListUsers(
		&iam.ListUsersInput{
			MaxItems: aws.Int64(1000),
		})

	if err != nil {
		log.Error(err)
		return nil
	}

	var users []User

	for _, user := range result.Users {
		tags := getUserTags(iamSession, *user.UserName)
		essentialTags := common.GetEssentialTags(tags, tagName)
		newUser := User{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *user.UserName,
				Description:  "User: " + *user.UserName,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
		}

		users = append(users, newUser)

	}

	return users
}

func getUserTags(iamSession *iam.IAM, roleName string) []*iam.Tag {
	tags, err := iamSession.ListUserTags(
		&iam.ListUserTagsInput{
			UserName: aws.String(roleName),
		})

	if err != nil {
		log.Error(err)
		return nil
	}

	return tags.Tags
}

func getUserAccessKeysIds(iamSession *iam.IAM, userName string) []*string {
	result, err := iamSession.ListAccessKeys(
		&iam.ListAccessKeysInput{
			UserName: aws.String(userName),
		})

	if err != nil {
		log.Error(err)
		return nil
	}

	var accessKeysIds []*string
	for _, accessKey := range result.AccessKeyMetadata {
		accessKeysIds = append(accessKeysIds, accessKey.AccessKeyId)
	}

	return accessKeysIds
}

func deleteUserAccessKey(iamSession *iam.IAM, userName string, accessKeyId string) {
	_, err := iamSession.DeleteAccessKey(
		&iam.DeleteAccessKeyInput{
			UserName:    aws.String(userName),
			AccessKeyId: aws.String(accessKeyId),
		})

	if err != nil {
		log.Errorf("Can't delete access key %s : %s", accessKeyId, err.Error())
	}
}

func deleteExpiredUserAccessKeys(iamSession *iam.IAM, userName string) {
	accessKeysIds := getUserAccessKeysIds(iamSession, userName)

	for _, accessKeyId := range accessKeysIds {
		deleteUserAccessKey(iamSession, userName, *accessKeyId)
	}
}

func DeleteExpiredUsers(iamSession *iam.IAM, options *AwsOptions) {
	users := getUsers(iamSession, options.TagName)
	var expiredUsers []User

	for _, user := range users {
		if user.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredUsers = append(expiredUsers, user)
		}
	}

	s := "There is no expired IAM user to delete."
	if len(expiredUsers) == 1 {
		s = "There is 1 expired IAM user to delete."
	}
	if len(expiredUsers) > 1 {
		s = fmt.Sprintf("There are %d expired IAM users to delete.", len(expiredUsers))
	}

	log.Debug(s)

	if options.DryRun || len(expiredUsers) == 0 {
		return
	}

	log.Debug("Starting expired IAM users deletion.")

	for _, user := range expiredUsers {
		HandleUserPolicies(iamSession, user.Identifier)
		deleteExpiredUserAccessKeys(iamSession, user.Identifier)

		_, userErr := iamSession.DeleteUser(
			&iam.DeleteUserInput{
				UserName: aws.String(user.Identifier),
			})
		if userErr != nil {
			log.Errorf("Can't delete user %s : %s", user.Identifier, userErr.Error())
		}
	}
}
