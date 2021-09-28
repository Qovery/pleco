package k8s

import (
	"context"
	"github.com/Qovery/pleco/pkg/common"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strconv"
	"time"
)

type kubernetesNamespace struct {
	Name                string
	NamespaceCreateTime time.Time
	Status              string
	TTL                 int64
}

func listNamespaces(clientSet *kubernetes.Clientset, tagName string) []v1.Namespace {
	listOptions := metav1.ListOptions{
		LabelSelector: tagName,
	}

	log.Debugf("Listing all Kubernetes namespaces with %s label", tagName)
	namespaces, err := clientSet.CoreV1().Namespaces().List(context.TODO(), listOptions)
	if err != nil {
		log.Errorf("Can'list namespaces with %s label: %s", tagName, err.Error())
	}

	return namespaces.Items
}

func getExpiredNamespaces(clientSet *kubernetes.Clientset, tagName string) []kubernetesNamespace {
	namespaces := listNamespaces(clientSet, tagName)

	expiredNamespaces := []kubernetesNamespace{}
	for _, namespace := range namespaces {
		if namespace.Status.Phase != "Active" {
			continue
		}

		for key, value := range namespace.ObjectMeta.Labels {
			if key == tagName {
				ttlValue, err := strconv.Atoi(value)

				if err != nil {
					log.Errorf("ttl value unrecognized for namespace %s", namespace.Name)
					continue
				}
				creationDate, _ := time.Parse(time.RFC3339, namespace.CreationTimestamp.Time.Format(time.RFC3339))
				if common.CheckIfExpired(creationDate, int64(ttlValue), "Namespace: "+namespace.Name) {
					expiredNamespaces = append(expiredNamespaces, kubernetesNamespace{
						Name:                namespace.Name,
						NamespaceCreateTime: namespace.CreationTimestamp.Time,
						Status:              string(namespace.Status.Phase),
						TTL:                 int64(ttlValue),
					})
				}
			}
		}
	}

	return expiredNamespaces
}

func deleteNamespace(clientSet *kubernetes.Clientset, namespace kubernetesNamespace, dryRun bool) {
	deleteOptions := metav1.DeleteOptions{}

	log.Debugf("Deleting namespace %s, expired after %d seconds", namespace.Name, namespace.TTL)
	if !dryRun {
		err := clientSet.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, deleteOptions)
		if err != nil {
			log.Errorf("Can't delete namsespace %s", namespace.Name)
		}
	}

}

func DeleteExpiredNamespaces(clientSet *kubernetes.Clientset, tagName string, dryRun bool) {
	namespaces := getExpiredNamespaces(clientSet, tagName)

	count, start := common.ElemToDeleteFormattedInfos("expired Kubernetes namespace", len(namespaces), "")

	log.Debug(count)

	if dryRun || len(namespaces) == 0 {
		return
	}

	log.Debug(start)

	for _, namespace := range namespaces {
		deleteNamespace(clientSet, namespace, dryRun)
	}
}
