package eks

import (
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
	log.Info("Starting ECR scan.")
	repositories := getRepositories(ecrSession)

	var emptyRepositoryNames []string
	for _, repository := range repositories {
		images := getRepositoryImages(ecrSession, *repository.RepositoryName)
		if len(images) == 0 {
			emptyRepositoryNames = append(emptyRepositoryNames, *repository.RepositoryName)
		}
	}

	s := "repository"
	if len(emptyRepositoryNames) > 1 {
		s = "repositories"
	}

	log.Infof("There is %d empty %s to delete.", len(emptyRepositoryNames), s)

	if drynRun {
		return
	}

	log.Info("Starting ECR deletion.")

	for _, repositoryName := range emptyRepositoryNames {
		_, err := ecrSession.DeleteRepository(
			&ecr.DeleteRepositoryInput{
				RepositoryName: aws.String(repositoryName),
			})

		if err != nil {
			log.Error(err)
		}
	}

}