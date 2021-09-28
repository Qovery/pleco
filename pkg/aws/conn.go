package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/sirupsen/logrus"
)

func CreateSession(region string) *session.Session {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		logrus.Fatalf("Can't connect to AWS: %s", err.Error())
	}
	return sess
}

func CreateSessionWithoutRegion() (*session.Session, error) {
	sess, err := session.NewSession()
	if err != nil {
		logrus.Errorf("Can't connect to AWS: %s", err)
		return nil, err
	}
	return sess, nil
}
