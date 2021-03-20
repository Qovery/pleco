module github.com/Qovery/pleco/cmd

go 1.16

replace github.com/Qovery/pleco/providers/aws => ../providers/aws

replace github.com/Qovery/pleco/providers/k8s => ../providers/k8s

require (
	github.com/Qovery/pleco/providers/aws v0.0.0-00010101000000-000000000000
	github.com/Qovery/pleco/providers/k8s v0.0.0-00010101000000-000000000000
	github.com/aws/aws-sdk-go v1.38.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.1
	k8s.io/client-go v0.20.5 // indirect
	sigs.k8s.io/aws-iam-authenticator v0.5.2 // indirect
)
