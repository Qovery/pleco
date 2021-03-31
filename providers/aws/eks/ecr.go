package eks

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	log "github.com/sirupsen/logrus"
)

func getRepositories(ecrSession *ecr.ECR) []*ecr.Repository {
	result, err := ecrSession.DescribeRepositories(
		&ecr.DescribeRepositoriesInput{
			MaxResults: aws.Int64(1000),
		})

	if err != nil {
		log.Error(err)
	}

	return result.Repositories
}

func getRepositoryImages(ecrSession *ecr.ECR, repositoryName string) []*ecr.ImageDetail {
	result, err := ecrSession.DescribeImages(
		&ecr.DescribeImagesInput{
			MaxResults: aws.Int64(1000),
			RepositoryName: aws.String(repositoryName),
		})

	if err != nil {
		log.Error(err)
	}

	return result.ImageDetails
}

func DeleteEmptyRepositories(ecrSession *ecr.ECR, drynRun bool) {
	repositories := getRepositories(ecrSession)
	region := ecrSession.Config.Region
	var emptyRepositoryNames []string
	for _, repository := range repositories {
		images := getRepositoryImages(ecrSession, *repository.RepositoryName)
		if len(images) == 0 {
			emptyRepositoryNames = append(emptyRepositoryNames, *repository.RepositoryName)
		}
	}

	s := fmt.Sprintf("There is no empty ECR repository to delete in region %s.", *region)
	if len(emptyRepositoryNames) == 1 {
		s = fmt.Sprintf("There is 1 empty ECR repository to delete in region %s.", *region)
	}
	if len(emptyRepositoryNames) > 1 {
		s = fmt.Sprintf("There are %d empty ECR repositories to delete in region %s.", len(emptyRepositoryNames), *region)
	}

	log.Debug(s)

	if drynRun || len(emptyRepositoryNames) == 0 {
		return
	}

	log.Debugf("Starting ECR repositories deletion for region %s.", *region)

	for _, repositoryName := range emptyRepositoryNames {
		_, err := ecrSession.DeleteRepository(
			&ecr.DeleteRepositoryInput{
				RepositoryName: aws.String(repositoryName),
			})

		if err != nil {
			log.Errorf("Deletion ECR repository error %s/%s: %s",
				repositoryName, *region, err)
		}
	}
}