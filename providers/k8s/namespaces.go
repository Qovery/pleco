package k8s

import (
	"context"
	"fmt"
	"github.com/Qovery/pleco/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strconv"
	"time"
)

type kubernetesNamespace struct {
	Name string
	NamespaceCreateTime time.Time
	Status string
	TTL int64
}

func listTaggedNamespaces(clientSet *kubernetes.Clientset, tagName string) ([]kubernetesNamespace, error) {
	var taggedNamespaces []kubernetesNamespace

	listOptions := metav1.ListOptions{
		LabelSelector:       tagName,
	}

	log.Debugf("Listing all Kubernetes namespaces with %s label", tagName)
	namespaces, err := clientSet.CoreV1().Namespaces().List(context.TODO(), listOptions)
	if err != nil {
		return taggedNamespaces, err
	}

	if len(namespaces.Items) == 0 {
		log.Debug("No Kubernetes namespaces with ttl were found")
		return taggedNamespaces, nil
	}

	for _, namespace := range namespaces.Items {
		for key, value := range namespace.ObjectMeta.Labels {
			if key == tagName {
				ttlValue, err := strconv.Atoi(value)

				if err != nil {
					log.Errorf("ttl value unrecognized for namespace %s", namespace.Name)
					continue
				}

				taggedNamespaces = append(taggedNamespaces, kubernetesNamespace{
					Name:                namespace.Name,
					NamespaceCreateTime: namespace.CreationTimestamp.Time,
					Status:              string(namespace.Status.Phase),
					TTL:                 int64(ttlValue),
				})
			}
		}
	}

	return taggedNamespaces, nil
}

func deleteNamespace(clientSet *kubernetes.Clientset, namespace kubernetesNamespace, dryRun bool) error {
	deleteOptions := metav1.DeleteOptions{}

	if namespace.Status == "Terminating" {
		log.Infof("Namespace %s is already in Terminating state, skipping...", namespace.Name)
		return nil
	} else if namespace.Status != "Active" {
		log.Warnf("Can't delete namespace %s because it is in %s state", namespace.Name, namespace.Status)
		return nil
	}

	log.Infof("Deleting namespace %s, expired after %d seconds", namespace.Name, namespace.TTL)
	if !dryRun {
		err := clientSet.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, deleteOptions)
		if err != nil {
			return err
		}
	}

	return nil
}

func DeleteExpiredNamespaces(clientSet *kubernetes.Clientset, tagName string, dryRun bool) error {

	namespaces, err := listTaggedNamespaces(clientSet, tagName)
	if err != nil {
		return fmt.Errorf("can't list kubernetes namespaces: %s\n", err)
	}

	for _, namespace := range namespaces {
		if utils.CheckIfExpired(namespace.NamespaceCreateTime, namespace.TTL) {
			err := deleteNamespace(clientSet, namespace, dryRun)
			if err != nil {
				log.Errorf("error while trying to delete namespace: %s", err)
			}
		}
	}

	return nil
}