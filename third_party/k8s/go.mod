module github.com/Qovery/pleco/third_party/k8s

go 1.16

replace github.com/Qovery/pleco/utils => ../../utils

require (
	github.com/Qovery/pleco/utils v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.3
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
)
