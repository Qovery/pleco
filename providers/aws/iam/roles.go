package iam

import (
	"fmt"
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
	"time"
)

type Role struct {
	RoleName        string
	CreationDate    time.Time
	ttl             int64
	Tag             string
	InstanceProfile []*iam.InstanceProfile
	IsProtected     bool
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
		instanceProfiles := getRoleInstanceProfile(iamSession, *role.RoleName)
		_, ttl, isProtected, _, _ := utils.GetEssentialTags(tags, tagName)
		newRole := Role{
			RoleName: *role.RoleName,
			CreationDate: *role.CreateDate,
			InstanceProfile: instanceProfiles,
			ttl: ttl,
			IsProtected: isProtected,
		}


			roles = append(roles, newRole)
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

func getRoleInstanceProfile(iamSession *iam.IAM, roleName string) []*iam.InstanceProfile{
	result, err := iamSession.ListInstanceProfilesForRole(
		&iam.ListInstanceProfilesForRoleInput{
			MaxItems: aws.Int64(1000),
			RoleName: aws.String(roleName),
		})

	if err != nil {
		log.Errorf("Can't get instance profiles for role %s : %s", roleName, err)
	}

	return result.InstanceProfiles
}



func DeleteExpiredRoles(iamSession *iam.IAM, tagName string, dryRun bool) {
	roles := getRoles(iamSession, tagName)
	var expiredRoles []Role

	for _, role := range roles {
		if utils.CheckIfExpired(role.CreationDate, role.ttl) && !role.IsProtected {
			expiredRoles = append(expiredRoles, role)
		}
	}

	s := "There is no expired IAM role to delete."
	if len(expiredRoles) == 1 {
		s = "There is 1 expired IAM role to delete."
	}
	if len(expiredRoles) > 1 {
		s = fmt.Sprintf("There are %d expired IAM roles to delete.", len(expiredRoles))
	}

	log.Debug(s)

	if dryRun || len(expiredRoles) == 0 {
		return
	}

	log.Debug("Starting expired IAM roles deletion.")


	for _, role := range expiredRoles {
		HandleRolePolicies(iamSession, role.RoleName)
		removeRoleFromInstanceProfile(iamSession, role.InstanceProfile, role.RoleName)

		_, err := iamSession.DeleteRole(
			&iam.DeleteRoleInput{
				RoleName: aws.String(role.RoleName),
			})

		if err != nil {
			log.Errorf("Can't delete role %s : %s", role.RoleName, err)
			}
	}
}

//func deleteRoleInstanceProfiles(iamSession *iam.IAM, roleInstanceProfiles []*iam.InstanceProfile) {
//	for _, instanceProfile := range roleInstanceProfiles {
//		_, err := iamSession.DeleteInstanceProfile(
//			&iam.DeleteInstanceProfileInput{
//				InstanceProfileName: aws.String(*instanceProfile.InstanceProfileName),
//			})
//
//		if err != nil {
//			log.Errorf("Can't delete instance profile %s : %s", *instanceProfile.InstanceProfileName, err)
//		}
//	}
//
//}

func removeRoleFromInstanceProfile(iamSession *iam.IAM, roleInstanceProfiles []*iam.InstanceProfile, roleName string) {
	for _, instanceProfile := range roleInstanceProfiles {
		_, err := iamSession.RemoveRoleFromInstanceProfile(
			&iam.RemoveRoleFromInstanceProfileInput{
				InstanceProfileName: aws.String(*instanceProfile.InstanceProfileName),
				RoleName: aws.String(roleName),
			})

		if err != nil {
			log.Errorf("Can't remove instance profile %s for role %s : %s", *instanceProfile.InstanceProfileName, roleName,  err)
		}
	}
}