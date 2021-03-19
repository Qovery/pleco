package iam

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

type User struct {
	UserName string
	CreationDate time.Time
	ttl int64
	Tag string
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
		tags := getUserRoles(iamSession, *user.UserName)
		newUser := User{
			UserName: *user.UserName,
			CreationDate: *user.CreateDate,
		}

		for _, tag := range tags {
			if strings.EqualFold(*tag.Key, tagName) {
				newUser.Tag = *tag.Value
			}

			if strings.EqualFold(*tag.Key, "ttl") {
				ttl, _ := strconv.ParseInt(*tag.Value, 10,64)
				newUser.ttl = ttl
			}
		}

		if newUser.ttl != 0 {
			users = append(users, newUser)
		}
	}

	return users
}

func getUserRoles(iamSession *iam.IAM, roleName string) []*iam.Tag {
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

func DeleteExpiredUsers(iamSession *iam.IAM, tagName string, dryRun bool) error {
	users := getUsers(iamSession, tagName)
	var expiredUsers []User

	for _, user := range users {
		if utils.CheckIfExpired(user.CreationDate, user.ttl) {
			expiredUsers = append(expiredUsers, user)
		}
	}

	log.Info("There is " + strconv.FormatInt(int64(len(expiredUsers)), 10) + " expired user(s) to delete.")

	if dryRun {
		return nil
	}

	for _, user := range expiredUsers {
		_, err := iamSession.DeleteUser(
			&iam.DeleteUserInput{
				UserName: aws.String(user.UserName),
			})
		if err != nil {
			return err
		}
	}

	return nil
}
