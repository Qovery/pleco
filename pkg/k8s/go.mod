module github.com/Qovery/pleco/pkg/k8s

go 1.16

replace github.com/Qovery/pleco/pkg/common => ../common

require (
	github.com/Qovery/pleco/pkg/common v0.0.0-00010101000000-000000000000
	github.com/aws/aws-sdk-go v1.40.49 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
)
