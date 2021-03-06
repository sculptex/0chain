module 0chain.net/core

go 1.14

replace 0chain.net/core => ../core

replace 0chain.net/smartcontract => ../smartcontract

replace 0chain.net/chaincore => ../chaincore

replace 0chain.net/conductor => ../conductor

require (
	0chain.net/chaincore v0.0.0
	github.com/0chain/gorocksdb v0.0.0-20181010114359-8752a9433481
	github.com/bitly/go-hostpool v0.0.0-20171023180738-a3a6125de932 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/didip/tollbooth v4.0.0+incompatible
	github.com/facebookgo/ensure v0.0.0-20160127193407-b4ab57deab51 // indirect
	github.com/facebookgo/stack v0.0.0-20160209184415-751773369052 // indirect
	github.com/facebookgo/subset v0.0.0-20150612182917-8dac2c3c4870 // indirect
	github.com/gocql/gocql v0.0.0-20190423091413-b99afaf3b163
	github.com/golang/snappy v0.0.1
	github.com/gomodule/redigo v2.0.0+incompatible
	github.com/hashicorp/golang-lru v0.5.1
	github.com/herumi/bls v0.0.0-20190423083323-d414f74643cb
	github.com/herumi/mcl v0.0.0-20190422075523-7f408a29acdc
	github.com/koding/cache v0.0.0-20161222233018-4a3175c6b2fe
	github.com/kr/pretty v0.1.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pelletier/go-toml v1.3.0 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.3.0
	github.com/valyala/gozstd v1.4.1
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20200323165209-0ec3e9974c59
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)
