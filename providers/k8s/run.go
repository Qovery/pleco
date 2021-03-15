package k8s

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"sync"
	"time"
)


func RunPlecoKubernetes(cmd *cobra.Command, interval int64, dryRun bool, wg *sync.WaitGroup) {
	wg.Add(1)
	go runPlecoOnKube(cmd, interval, dryRun, wg)
}

func runPlecoOnKube(cmd *cobra.Command, interval int64, dryRun bool, wg *sync.WaitGroup) {
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

	// check Kubernetes
	for {
		if kubernetesEnabled {
			err := DeleteExpiredNamespaces(k8sClientSet, tagName, dryRun)
			if err != nil {
				logrus.Error(err)
			}
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}

}