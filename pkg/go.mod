module github.com/Qovery/pleco/pkg

go 1.16

replace (
	github.com/Qovery/pleco/third_party/aws => ../third_party/aws
	github.com/Qovery/pleco/third_party/k8s => ../third_party/k8s
	github.com/Qovery/pleco/third_party/scaleway => ../third_party/scaleway
	github.com/Qovery/pleco/utils => ../utils
)

require (
	github.com/Qovery/pleco/third_party/aws v0.0.0-00010101000000-000000000000
	github.com/Qovery/pleco/third_party/k8s v0.0.0-00010101000000-000000000000
	github.com/Qovery/pleco/third_party/scaleway v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
)
