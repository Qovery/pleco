module github.com/Qovery/pleco/core

go 1.16

replace (
	github.com/Qovery/pleco/providers/aws => ../providers/aws
	github.com/Qovery/pleco/providers/k8s => ../providers/k8s
	github.com/Qovery/pleco/utils => ../utils
)

require (
	github.com/Qovery/pleco/providers/aws v0.0.0-00010101000000-000000000000
	github.com/Qovery/pleco/providers/k8s v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
)
