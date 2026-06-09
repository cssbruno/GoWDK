module github.com/cssbruno/gowdk/runtime/contracts/redisstream

go 1.26.4

require (
	github.com/cssbruno/gowdk v0.2.7
	github.com/redis/go-redis/v9 v9.20.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
)

replace github.com/cssbruno/gowdk => ../../..
