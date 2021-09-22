module github.com/Qovery/pleco/third_party/scaleway

go 1.16

//
//replace github.com/Qovery/pleco/utils => ../../utils

require (
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.7
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
)
