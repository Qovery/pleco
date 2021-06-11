module github.com/Qovery/pleco/providers/aws

go 1.16

replace github.com/Qovery/pleco/utils => ../../utils

require (
	github.com/Qovery/pleco/utils v0.0.0-00010101000000-000000000000
	github.com/aws/aws-sdk-go v1.38.53
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.3
	k8s.io/client-go v0.21.1
	sigs.k8s.io/aws-iam-authenticator v0.5.3
)
