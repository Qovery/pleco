module github.com/Qovery/pleco

go 1.16

replace (
	github.com/Qovery/pleco/cmd => ./cmd
	github.com/Qovery/pleco/pkg => ./pkg
)

require github.com/Qovery/pleco/cmd v0.0.0-00010101000000-000000000000
