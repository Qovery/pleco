package k8s

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"sync"
	"time"
)

var wg sync.WaitGroup

func RunPlecoKubernetes(cmd *cobra.Command, interval int64, dryRun bool) {
	wg.Add(1)
	go runPlecoOnKube(cmd, interval, dryRun)
}

func runPlecoOnKube(cmd *cobra.Command, interval int64, dryRun bool) {
	defer wg.Done()

	// Kubernetes connection
	var k8sClientSet *kubernetes.Clientset
	var err error
	kubernetesEnabled := true
	KubernetesConn, _ := cmd.Flags().GetString("kube-conn")
	tagName, _ := cmd.Flags().GetString("tag-name")

	switch KubernetesConn {
	case "in":
		k8sClientSet, err = AuthenticateInCluster()
	case "out":
		k8sClientSet, err = AuthenticateOutOfCluster()
	default:
		kubernetesEnabled = false
	}
	if err != nil {
		logrus.Errorf("failed to authenticate on kubernetes with %s connection: %v", KubernetesConn, err)
	}

	for {
		// check Kubernetes
		if kubernetesEnabled {
			err := DeleteExpiredNamespaces(k8sClientSet, tagName, dryRun)
			if err != nil {
				logrus.Error(err)
			}
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}

	wg.Wait()
}