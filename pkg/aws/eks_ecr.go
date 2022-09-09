package aws

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

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

func DeleteEmptyRepositories(sessions AWSSessions, options AwsOptions) {
	repositories := getRepositories(sessions.ECR)
	region := sessions.ECR.Config.Region
	var emptyRepositoryNames []string
	for _, repository := range repositories {
		time, _ := time.Parse(time.RFC3339, repository.CreatedAt.Format(time.RFC3339))
		result, err := sessions.ECR.ListTagsForResource(&ecr.ListTagsForResourceInput{ResourceArn: repository.RepositoryArn})
		if err != nil {
			log.Error(err)
		}

		tags := common.GetEssentialTags(result.Tags, options.TagName)

		if common.CheckIfExpired(time, tags.TTL, fmt.Sprintf("ECR repository: %s", *repository.RepositoryName), options.DisableTTLCheck) {
			images := getRepositoryImages(sessions.ECR, *repository.RepositoryName)

			if len(images) == 0 {
				emptyRepositoryNames = append(emptyRepositoryNames, *repository.RepositoryName)
			}
		}

	}

	s := fmt.Sprintf("There is no empty ECR repository to delete in region %s.", *region)
	if len(emptyRepositoryNames) == 1 {
		s = fmt.Sprintf("There is 1 empty ECR repository to delete in region %s.", *region)
	}
	if len(emptyRepositoryNames) > 1 {
		s = fmt.Sprintf("There are %d empty ECR repositories to delete in region %s.", len(emptyRepositoryNames), *region)
	}

	log.Info(s)

	if options.DryRun || len(emptyRepositoryNames) == 0 {
		return
	}

	log.Infof("Starting ECR repositories deletion for region %s.", *region)

	for _, repositoryName := range emptyRepositoryNames {
		_, err := sessions.ECR.DeleteRepository(
			&ecr.DeleteRepositoryInput{
				RepositoryName: aws.String(repositoryName),
			})

		if err != nil {
			log.Errorf("Deletion ECR repository error %s/%s: %s",
				repositoryName, *region, err)
		} else {
			log.Debugf("ECR %s in %s deleted.", repositoryName, *region)
		}
	}
}
