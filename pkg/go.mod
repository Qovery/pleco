module github.com/Qovery/pleco/pkg

go 1.16

replace (
	github.com/Qovery/pleco/pkg/aws => ./aws
	github.com/Qovery/pleco/pkg/common => ./common
	github.com/Qovery/pleco/pkg/k8s => ./k8s
	github.com/Qovery/pleco/pkg/scaleway => ./scaleway
)

require (
	github.com/Qovery/pleco/pkg/aws v0.0.0-00010101000000-000000000000
	github.com/Qovery/pleco/pkg/common v0.0.0-00010101000000-000000000000
	github.com/Qovery/pleco/pkg/k8s v0.0.0-00010101000000-000000000000
	github.com/Qovery/pleco/pkg/scaleway v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
)
