module github.com/threefoldtech/grid3-go

go 1.16

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/centrifuge/go-substrate-rpc-client/v4 v4.0.2
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.26.0
	github.com/stretchr/testify v1.7.0
	github.com/threefoldtech/go-rmb v0.2.0
	github.com/threefoldtech/grid_proxy_server v1.5.6
	github.com/threefoldtech/substrate-client v0.0.0-20220808155028-1d74b8477705
	github.com/threefoldtech/zos v0.5.6-0.20220804142531-495bf966448a
)

replace github.com/centrifuge/go-substrate-rpc-client/v4 v4.0.2 => github.com/threefoldtech/go-substrate-rpc-client/v4 v4.0.3-0.20220629145942-1ef6a654b4b5
