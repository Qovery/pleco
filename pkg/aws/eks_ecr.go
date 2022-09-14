package aws

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type Repository struct {
	name      string
	imagesIds []*ecr.ImageIdentifier
}

func getRepositories(ecrSession *ecr.ECR) []*ecr.Repository {
	var repo []*ecr.Repository

	var token *string
	for {
		result, err := ecrSession.DescribeRepositories(
			&ecr.DescribeRepositoriesInput{
				MaxResults: aws.Int64(1000),
				NextToken:  token,
			})

		if err != nil {
			log.Error(err)
		}

		token = result.NextToken
		repo = append(repo, result.Repositories...)

		if result.NextToken == nil {
			break
		}
	}

	return repo
}

func getRepositoryImages(ecrSession *ecr.ECR, repositoryName string) []*ecr.ImageDetail {
	result, err := ecrSession.DescribeImages(
		&ecr.DescribeImagesInput{
			MaxResults:     aws.Int64(1000),
			RepositoryName: aws.String(repositoryName),
		})

	if err != nil {
		log.Error(err)
	}

	return result.ImageDetails
}

func DeleteExpiredRepositories(sessions AWSSessions, options AwsOptions) {
	repositories := getRepositories(sessions.ECR)
	region := sessions.ECR.Config.Region
	var expiredRepository []Repository
	for _, repository := range repositories {
		time, _ := time.Parse(time.RFC3339, repository.CreatedAt.Format(time.RFC3339))
		result, err := sessions.ECR.ListTagsForResource(&ecr.ListTagsForResourceInput{ResourceArn: repository.RepositoryArn})
		if err != nil {
			log.Error(err)
		}

		imagesIds := getRepositoryImageIds(sessions.ECR, *repository.RepositoryName)
		if len(imagesIds) == 0 {
			expiredRepository = append(expiredRepository, Repository{
				name:      *repository.RepositoryName,
				imagesIds: imagesIds,
			})
			continue
		}

		tags := common.GetEssentialTags(result.Tags, options.TagName)
		if common.CheckIfExpired(time, tags.TTL, fmt.Sprintf("ECR repository: %s", *repository.RepositoryName), options.DisableTTLCheck) {
			expiredRepository = append(expiredRepository, Repository{
				name:      *repository.RepositoryName,
				imagesIds: imagesIds,
			})
		}
	}

	s := fmt.Sprintf("There is no expired ECR repository to delete in region %s.", *region)
	if len(expiredRepository) == 1 {
		s = fmt.Sprintf("There is 1 expired ECR repository to delete in region %s.", *region)
	}
	if len(expiredRepository) > 1 {
		s = fmt.Sprintf("There are %d expired ECR repositories to delete in region %s.", len(expiredRepository), *region)
	}

	log.Info(s)

	if options.DryRun || len(expiredRepository) == 0 {
		return
	}

	log.Infof("Starting ECR repositories deletion for region %s.", *region)

	for _, reposirory := range expiredRepository {
		deleteRepository(sessions.ECR, reposirory)
	}
}

func emptyRepository(ecrSession *ecr.ECR, repositoryName *string, imageIds []*ecr.ImageIdentifier) {
	_, err := ecrSession.BatchDeleteImage(&ecr.BatchDeleteImageInput{RepositoryName: repositoryName, ImageIds: imageIds})
	if err != nil {
		log.Error(err)
	}
}

func getRepositoryImageIds(ecrSession *ecr.ECR, repositoryName string) []*ecr.ImageIdentifier {
	result, err := ecrSession.ListImages(&ecr.ListImagesInput{RepositoryName: &repositoryName})
	if err != nil {
		log.Error(err)
	}
	return result.ImageIds
}

func deleteRepository(ecrSession *ecr.ECR, repository Repository) {
	emptyRepository(ecrSession, &repository.name, repository.imagesIds)
	_, err := ecrSession.DeleteRepository(
		&ecr.DeleteRepositoryInput{
			RepositoryName: aws.String(repository.name),
		})

	if err != nil {
		log.Errorf("Deletion ECR repository error %s/%s: %s",
			repository.name, *ecrSession.Config.Region, err)
	} else {
		log.Debugf("ECR %s in %s deleted.", repository.name, *ecrSession.Config.Region)
	}
}
