package k8s

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"github.com/Qovery/pleco/pkg/common"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type kubernetesNamespace struct {
	Name                string
	NamespaceCreateTime time.Time
	Status              string
	TTL                 int64
}

func listNamespaces(clientSet *kubernetes.Clientset, tagName string, disableTTLCheck bool) []v1.Namespace {
	var listOptions metav1.ListOptions
	if !disableTTLCheck {
		listOptions = metav1.ListOptions{
			LabelSelector: tagName,
		}
	}

	log.Debugf("Listing all Kubernetes namespaces with %s label", tagName)
	namespaces, err := clientSet.CoreV1().Namespaces().List(context.TODO(), listOptions)
	if err != nil {
		log.Errorf("Can't list namespaces with %s label: %s", tagName, err.Error())
	}

	return namespaces.Items
}

func getExpiredNamespaces(clientSet *kubernetes.Clientset, tagName string, disableTTLCheck bool) []kubernetesNamespace {
	namespaces := listNamespaces(clientSet, tagName, disableTTLCheck)

	expiredNamespaces := []kubernetesNamespace{}
	for _, namespace := range namespaces {
		if namespace.Status.Phase != "Active" {
			continue
		}

		if disableTTLCheck {
			match, _ := regexp.Compile("z([a-z0-9]+)-z(([a-z0-9]+))")
			if !match.MatchString(namespace.Name) {
				continue
			}
		}

		for key, value := range namespace.ObjectMeta.Labels {
			if key == tagName || disableTTLCheck {
				ttlValue, err := strconv.Atoi(value)

				if err != nil && !disableTTLCheck {
					log.Errorf("ttl value unrecognized for namespace %s", namespace.Name)
					continue
				}

				if disableTTLCheck {
					ttlValue = -1
				}
				creationDate, _ := time.Parse(time.RFC3339, namespace.CreationTimestamp.Time.Format(time.RFC3339))
				if common.CheckIfExpired(creationDate, int64(ttlValue), "Namespace: "+namespace.Name, disableTTLCheck) {
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

	if !dryRun {
		err := clientSet.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, deleteOptions)
		if err != nil {
			log.Errorf("Can't delete namsespace %s", namespace.Name)
		} else {
			log.Debugf("K8S namespace %s deleted.", namespace.Name)
		}
	}

}

func DeleteExpiredNamespaces(clientSet *kubernetes.Clientset, tagName string, dryRun bool, disableTTLCheck bool) {
	namespaces := getExpiredNamespaces(clientSet, tagName, disableTTLCheck)

	count, start := common.ElemToDeleteFormattedInfos("expired Kubernetes namespace", len(namespaces), "")

	log.Info(count)

	if dryRun || len(namespaces) == 0 {
		return
	}

	log.Info(start)

	for _, namespace := range namespaces {
		deleteNamespace(clientSet, namespace, dryRun)
	}
}
