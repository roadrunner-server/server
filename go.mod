module github.com/roadrunner-server/server/v2

go 1.17

require (
	github.com/roadrunner-server/api/v2 v2.8.0
	github.com/roadrunner-server/errors v1.1.1
	github.com/roadrunner-server/sdk/v2 v2.8.1
	github.com/stretchr/testify v1.7.0
	go.uber.org/zap v1.21.0
)

replace github.com/roadrunner-server/sdk/v2 => ../../sdk

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/goccy/go-json v0.9.4 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/roadrunner-server/goridge/v3 v3.3.1 // indirect
	github.com/roadrunner-server/tcplisten v1.1.1 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tklauser/numcpus v0.4.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)
