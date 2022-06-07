package do

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/digitalocean/godo"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
)

func CreateSession() *godo.Client {
	return godo.NewFromToken(os.Getenv("DO_API_TOKEN"))
}

func CreateMinIOSession(region string) *minio.Client {
	endpoint := fmt.Sprintf("%s.digitaloceanspaces.com", region)
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("DO_SPACES_KEY"), os.Getenv("DO_SPACES_SECRET"), ""),
		Region: region,
	})
	if err != nil {
		log.Fatalln(err)
	}

	return minioClient
}

func volumeTimeout() time.Duration {
	env, ok := os.LookupEnv("DO_VOLUME_TIMEOUT")
	if ok {
		timeout, err := strconv.Atoi(env)
		if err != nil {
			logrus.Errorf("Can't parse VOLUME_TIMEOUT variable. Set to default (24 hours)")
			return 24
		}
		return time.Duration(timeout)
	}

	return 24
}
