module github.com/Qovery/pleco/cmd

go 1.16

replace (
	github.com/Qovery/pleco/pkg => ../pkg
	github.com/Qovery/pleco/pkg/aws => ../pkg/aws
	github.com/Qovery/pleco/pkg/common => ../pkg/common
	github.com/Qovery/pleco/pkg/k8s => ../pkg/k8s
	github.com/Qovery/pleco/pkg/scaleway => ../pkg/scaleway
)

require (
	github.com/Qovery/pleco/pkg v0.0.0-00010101000000-000000000000
	github.com/mitchellh/go-homedir v1.1.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.9.0
)
