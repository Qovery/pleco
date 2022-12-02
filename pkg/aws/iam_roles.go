package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type Role struct {
	common.CloudProviderResource
	InstanceProfile []*iam.InstanceProfile
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
		if strings.HasPrefix(*role.RoleName, "AWS") {
			continue
		}
		tags := getRoleTags(iamSession, *role.RoleName)
		instanceProfiles := getRoleInstanceProfile(iamSession, *role.RoleName)
		essentialTags := common.GetEssentialTags(tags, tagName)
		newRole := Role{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *role.RoleName,
				Description:  "IAM Role: " + *role.RoleName,
				CreationDate: role.CreateDate.UTC(),
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			InstanceProfile: instanceProfiles,
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

func getRoleInstanceProfile(iamSession *iam.IAM, roleName string) []*iam.InstanceProfile {
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

func DeleteExpiredRoles(sessions *AWSSessions, options *AwsOptions) {
	iamSession := sessions.IAM
	roles := getRoles(iamSession, options.TagName)
	var expiredRoles []Role

	for _, role := range roles {
		if role.CloudProviderResource.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
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

	log.Info(s)

	if options.DryRun || len(expiredRoles) == 0 {
		return
	}

	log.Info("Starting expired IAM roles deletion.")

	for _, role := range expiredRoles {
		HandleRolePolicies(iamSession, role.Identifier)
		removeRoleFromInstanceProfile(iamSession, role.InstanceProfile, role.Identifier)

		_, err := iamSession.DeleteRole(
			&iam.DeleteRoleInput{
				RoleName: aws.String(role.Identifier),
			})

		if err != nil {
			log.Errorf("Can't delete role %s : %s", role.Identifier, err)
		} else {
			log.Debugf("Iam Role %s in %s deleted.", role.Identifier, *iamSession.Config.Region)
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
				RoleName:            aws.String(roleName),
			})

		if err != nil {
			log.Errorf("Can't remove instance profile %s for role %s : %s", *instanceProfile.InstanceProfileName, roleName, err)
		} else {
			log.Debugf("Instance profile %s for role %s in %s removed.", *instanceProfile.InstanceProfileName, roleName, *iamSession.Config.Region)
		}
	}
}
