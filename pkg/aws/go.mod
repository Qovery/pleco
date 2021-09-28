module github.com/Qovery/pleco/pkg/aws

go 1.17

replace github.com/Qovery/pleco/pkg/common => ../common

require (
	github.com/Qovery/pleco/pkg/common v0.0.0-00010101000000-000000000000
	github.com/aws/aws-sdk-go v1.40.49
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	k8s.io/client-go v0.22.2
	sigs.k8s.io/aws-iam-authenticator v0.5.3
)
