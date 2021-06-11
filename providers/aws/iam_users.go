package aws

import (
	"fmt"
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
	"time"
)

type User struct {
	UserName     string
	CreationDate time.Time
	ttl          int64
	Tag          string
	IsProtected  bool
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
		_ , ttl, isProtected, _, _ := utils.GetEssentialTags(tags, tagName)
		newUser := User{
			UserName: *user.UserName,
			CreationDate: *user.CreateDate,
			ttl: ttl,
			IsProtected: isProtected,
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
			UserName: aws.String(userName),
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

func DeleteExpiredUsers(iamSession *iam.IAM, tagName string, dryRun bool) {
	users := getUsers(iamSession, tagName)
	var expiredUsers []User

	for _, user := range users {
		if utils.CheckIfExpired(user.CreationDate, user.ttl, "iam user: " + user.UserName) && !user.IsProtected {
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

	if dryRun || len(expiredUsers) == 0 {
		return
	}

	log.Debug("Starting expired IAM users deletion.")

	for _, user := range expiredUsers {
		HandleUserPolicies(iamSession, user.UserName)
		deleteExpiredUserAccessKeys(iamSession, user.UserName)

		_, userErr := iamSession.DeleteUser(
			&iam.DeleteUserInput{
				UserName: aws.String(user.UserName),
			})
		if userErr != nil {
			log.Errorf("Can't delete user %s : %s", user.UserName, userErr.Error())
		}
	}
}
