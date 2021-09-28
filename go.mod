module github.com/Qovery/pleco

go 1.17

replace (
	github.com/Qovery/pleco/cmd => ./cmd
	github.com/Qovery/pleco/pkg => ./pkg
	github.com/Qovery/pleco/pkg/aws => ./pkg/aws
	github.com/Qovery/pleco/pkg/common => ./pkg/common
	github.com/Qovery/pleco/pkg/k8s => ./pkg/k8s
	github.com/Qovery/pleco/pkg/scaleway => ./pkg/scaleway
)

require github.com/Qovery/pleco/cmd v0.0.0-00010101000000-000000000000
