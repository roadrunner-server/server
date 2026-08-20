module github.com/roadrunner-server/server/v6

go 1.26

toolchain go1.26.5

require (
	github.com/roadrunner-server/errors v1.5.0
	github.com/roadrunner-server/pool/v2 v2.0.0-beta.1
	github.com/roadrunner-server/tcplisten v1.5.2
	github.com/spf13/viper v1.21.0
	github.com/stretchr/testify v1.12.1
)

exclude (
	github.com/spf13/viper v1.18.0
	github.com/spf13/viper v1.18.1
)

require (
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/roadrunner-server/events v1.0.1 // indirect
	github.com/roadrunner-server/goridge/v4 v4.0.0-beta.3 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)
