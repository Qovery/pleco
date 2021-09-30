module github.com/Qovery/pleco/cmd

go 1.17

replace (
	github.com/Qovery/pleco/pkg => ../pkg
	github.com/Qovery/pleco/pkg/aws => ../pkg/aws
	github.com/Qovery/pleco/pkg/common => ../pkg/common
	github.com/Qovery/pleco/pkg/k8s => ../pkg/k8s
	github.com/Qovery/pleco/pkg/scaleway => ../pkg/scaleway
)

require (
	github.com/Qovery/pleco/pkg v0.0.0-00010101000000-000000000000
	github.com/Qovery/pleco/pkg/common v0.0.0-00010101000000-000000000000
	github.com/mitchellh/go-homedir v1.1.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.9.0
)

require (
	github.com/Qovery/pleco/pkg/aws v0.0.0-00010101000000-000000000000 // indirect
	github.com/Qovery/pleco/pkg/k8s v0.0.0-00010101000000-000000000000 // indirect
	github.com/Qovery/pleco/pkg/scaleway v0.0.0-00010101000000-000000000000 // indirect
	github.com/aws/aws-sdk-go v1.40.49 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/gofrs/flock v0.7.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/imdario/mergo v0.3.5 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/klauspost/cpuid v1.3.1 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/minio/md5-simd v1.1.0 // indirect
	github.com/minio/minio-go/v7 v7.0.14 // indirect
	github.com/minio/sha256-simd v0.1.1 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/rs/xid v1.2.1 // indirect
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.7 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f // indirect
	golang.org/x/sys v0.0.0-20210823070655-63515b42dcdf // indirect
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d // indirect
	golang.org/x/text v0.3.6 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.63.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.22.2 // indirect
	k8s.io/apimachinery v0.22.2 // indirect
	k8s.io/client-go v0.22.2 // indirect
	k8s.io/klog/v2 v2.9.0 // indirect
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a // indirect
	sigs.k8s.io/aws-iam-authenticator v0.5.3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)
