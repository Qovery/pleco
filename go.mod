module github.com/Qovery/pleco

go 1.16

replace (
	github.com/Qovery/pleco/cmd => ./cmd
	github.com/Qovery/pleco/pkg => ./pkg
	github.com/Qovery/pleco/third_party/aws => ./third_party/aws
	github.com/Qovery/pleco/third_party/k8s => ./third_party/k8s
	github.com/Qovery/pleco/third_party/scaleway => ./third_party/scaleway
	github.com/Qovery/pleco/utils => ./utils
)

require github.com/Qovery/pleco/cmd v0.0.0-00010101000000-000000000000
