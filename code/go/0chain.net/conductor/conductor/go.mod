module conductor

replace 0chain.net/conductor => ../../conductor

replace 0chain.net/chaincore => ../../chaincore

replace 0chain.net/core => ../../core

replace 0chain.net/smartcontract => ../../smartcontract

go 1.14

require (
	0chain.net/conductor v0.0.0-00010101000000-000000000000
	github.com/kr/pretty v0.2.0
	github.com/spf13/viper v1.7.0 // indirect
	github.com/valyala/gorpc v0.0.0-20160519171614-908281bef774 // indirect
	gopkg.in/yaml.v2 v2.3.0
)
