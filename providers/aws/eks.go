package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

func AuthenticateToEks(clusterName string, clusterUrl string, roleArn string, session *session.Session) (*kubernetes.Clientset, error) {
	clusterApi := &api.Cluster{Server: clusterUrl}
	clusters := make(map[string]*api.Cluster)
	clusters[clusterName] = clusterApi
	c := &api.Config{Clusters: clusters}

	g, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create iam-authenticator token generator: %v", err)
	}

	t, err := g.GetWithRoleForSession("eks_test", roleArn, session)
	if err != nil {
		return nil, fmt.Errorf("failed to get token for eks: %v", err)
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*c, &clientcmd.ConfigOverrides{Context: api.Context{Cluster: clusterName}, AuthInfo: api.AuthInfo{Token: t.Token}})
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %v", err)
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client set: %v", err)
	}
	return clientSet, nil
}