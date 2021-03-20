package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/prometheus/common/log"
	"strconv"
	"strings"
	"time"
)

func DeleteExpiredIAM(sessions *AWSSessions, options *AwsOption) error {
	err := DeleteExpiredUsers(sessions.IAM, options.TagName, options.DryRun)
	if err != nil {
		return err
	}

	err = DeleteExpiredRoles(sessions.IAM, options.TagName, options.DryRun)
	if err != nil {
		return err
	}

	err = DeleteDetachedPolicies(sessions.IAM, options.DryRun)
	if err != nil {
		return err
	}

	return nil
}

// GROUP

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

func DeleteGroups(iamSession *iam.IAM, dryRun bool) error {
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

// POLICIES

func getPolicies(iamSession *iam.IAM) []*iam.Policy {
	result, err := iamSession.ListPolicies(
		&iam.ListPoliciesInput{
			MaxItems: aws.Int64(1000),
		})

	if err != nil {
		log.Error(err)
		return nil
	}

	return result.Policies
}

func DeleteDetachedPolicies(iamSession *iam.IAM, dryRun bool) error {
	policies := getPolicies(iamSession)
	var detachedPolicies []*iam.Policy

	for _, policy := range policies {
		if *policy.AttachmentCount == 0 {
			detachedPolicies = append(detachedPolicies, policy)
		}
	}

	log.Info("There is " + strconv.FormatInt(int64(len(detachedPolicies)), 10) + " detached policies to delete.")

	if dryRun {
		return nil
	}

	for _, expiredPolicy := range detachedPolicies {
		_, err := iamSession.DeletePolicy(
			&iam.DeletePolicyInput{
				PolicyArn: aws.String(*expiredPolicy.Arn),
			})

		if err != nil {
			return err
		}
	}

	return nil
}

// ROLES

type Role struct {
	RoleName     string
	CreationDate time.Time
	ttl          int64
	Tag          string
}

func getRoles(iamSession *iam.IAM, tagName string) []Role {
	result, err := iamSession.ListRoles(
		&iam.ListRolesInput{
			MaxItems: aws.Int64(1000),
		})

	if err != nil {
		log.Error(err)
		return nil
	}

	var roles []Role

	for _, role := range result.Roles {
		tags := getRoleTags(iamSession, *role.RoleName)
		newRole := Role{
			RoleName:     *role.RoleName,
			CreationDate: *role.CreateDate,
		}

		for _, tag := range tags {
			if strings.EqualFold(*tag.Key, tagName) {
				newRole.Tag = *tag.Value
			}

			if strings.EqualFold(*tag.Key, "ttl") {
				ttl, _ := strconv.ParseInt(*tag.Value, 10, 64)
				newRole.ttl = ttl
			}
		}

		if newRole.ttl != 0 {
			roles = append(roles, newRole)
		}
	}

	return roles
}

func getRoleTags(iamSession *iam.IAM, roleName string) []*iam.Tag {
	tags, err := iamSession.ListRoleTags(
		&iam.ListRoleTagsInput{
			RoleName: aws.String(roleName),
		})

	if err != nil {
		log.Error(err)
		return nil
	}

	return tags.Tags
}

func DeleteExpiredRoles(iamSession *iam.IAM, tagName string, dryRun bool) error {
	roles := getRoles(iamSession, tagName)
	var expiredRoles []Role

	for _, role := range roles {
		if CheckIfExpired(role.CreationDate, role.ttl) {
			expiredRoles = append(expiredRoles, role)
		}
	}

	log.Info("There is " + strconv.FormatInt(int64(len(expiredRoles)), 10) + " expired role(s) to delete.")

	if dryRun {
		return nil
	}

	for _, role := range expiredRoles {
		_, err := iamSession.DeleteRole(
			&iam.DeleteRoleInput{
				RoleName: aws.String(role.RoleName)})
		if err != nil {
			return err
		}
	}

	return nil
}

// USERS

type User struct {
	UserName     string
	CreationDate time.Time
	ttl          int64
	Tag          string
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
			UserName:     *user.UserName,
			CreationDate: *user.CreateDate,
		}

		for _, tag := range tags {
			if strings.EqualFold(*tag.Key, tagName) {
				newUser.Tag = *tag.Value
			}

			if strings.EqualFold(*tag.Key, "ttl") {
				ttl, _ := strconv.ParseInt(*tag.Value, 10, 64)
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
		if CheckIfExpired(user.CreationDate, user.ttl) {
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
