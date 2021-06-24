module github.com/Qovery/pleco

go 1.16

replace github.com/Qovery/pleco/cmd => ./cmd

replace github.com/Qovery/pleco/core => ./core

replace github.com/Qovery/pleco/providers/aws => ./providers/aws

replace github.com/Qovery/pleco/providers/k8s => ./providers/k8s

replace github.com/Qovery/pleco/utils => ./utils

require (
	github.com/Qovery/pleco/cmd v0.0.0-00010101000000-000000000000 // indirect
	github.com/Qovery/pleco/core v0.0.0-00010101000000-000000000000 // indirect
	github.com/Qovery/pleco/providers/aws v0.0.0-00010101000000-000000000000 // indirect
	github.com/Qovery/pleco/providers/k8s v0.0.0-00010101000000-000000000000 // indirect
	github.com/spf13/viper v1.7.1 // indirect
)
