package k8s

import (
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"time"
)

type K8SOptions struct {
	TagName        string
	DryRun         bool
	ConnectionType string
}

func RunPlecoKubernetes(interval int64, options *K8SOptions) {
	runPlecoOnKube(interval, options)
}

func runPlecoOnKube(interval int64, options *K8SOptions) {
	// Kubernetes connection
	var k8sClientSet *kubernetes.Clientset
	var err error
	kubernetesEnabled := true

	switch options.ConnectionType {
	case "in":
		k8sClientSet, err = AuthenticateInCluster()
	case "out":
		k8sClientSet, err = AuthenticateOutOfCluster()
	default:
		kubernetesEnabled = false
	}
	if err != nil {
		logrus.Errorf("failed to authenticate on kubernetes with %s connection: %v", options.ConnectionType, err)
	}

	// check Kubernetes
	for {
		if kubernetesEnabled {
			// TODO : create an interface if more func is called like aws package
			err := DeleteExpiredNamespaces(k8sClientSet, options.TagName, options.DryRun)
			if err != nil {
				logrus.Error(err)
			}
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}

}
