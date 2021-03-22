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

type Role struct {
	RoleName string
	CreationDate time.Time
	ttl int64
	Tag string
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
			RoleName: *role.RoleName,
			CreationDate: *role.CreateDate,
		}

		for _, tag := range tags {
			if strings.EqualFold(*tag.Key, tagName) {
				newRole.Tag = *tag.Value
			}

			if strings.EqualFold(*tag.Key, "ttl") {
				ttl, _ := strconv.ParseInt(*tag.Value, 10,64)
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



func DeleteExpiredRoles(iamSession *iam.IAM, tagName string, dryRun bool) {
	roles := getRoles(iamSession, tagName)
	var expiredRoles []Role

	for _, role := range roles {
		if utils.CheckIfExpired(role.CreationDate, role.ttl) {
			expiredRoles = append(expiredRoles, role)
		}
	}

	log.Info("There is " + strconv.FormatInt(int64(len(expiredRoles)), 10) + " expired role(s) to delete.")

	if dryRun {
		return
	}

	log.Info("Starting expired roles deletion.")


	for _, role := range expiredRoles {
		HandleRolePolicies(iamSession, role.RoleName)

		_, err := iamSession.DeleteRole(
			&iam.DeleteRoleInput{
				RoleName: aws.String(role.RoleName),
			})

		if err != nil {
			log.Errorf("Can't delete role %s : %s", role.RoleName, err)
			}
	}
}