module github.com/Qovery/pleco/cmd

go 1.16

replace (
	github.com/Qovery/pleco/core => ../core
	github.com/Qovery/pleco/providers/aws => ../providers/aws
	github.com/Qovery/pleco/providers/k8s => ../providers/k8s
	github.com/Qovery/pleco/utils => ../utils
)

require (
	github.com/Qovery/pleco/core v0.0.0-00010101000000-000000000000
	github.com/mitchellh/go-homedir v1.1.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.8.1
)
