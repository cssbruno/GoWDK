package ratelimit

import runtimeratelimit "github.com/cssbruno/gowdk/runtime/ratelimit"

type ErrorHandler = runtimeratelimit.ErrorHandler
type InMemoryOptions = runtimeratelimit.InMemoryOptions
type InMemoryStore = runtimeratelimit.InMemoryStore
type KeyFunc = runtimeratelimit.KeyFunc
type LimitHandler = runtimeratelimit.LimitHandler
type Limiter = runtimeratelimit.Limiter
type Options = runtimeratelimit.Options
type RedisClient = runtimeratelimit.RedisClient
type RedisOptions = runtimeratelimit.RedisOptions
type RedisStore = runtimeratelimit.RedisStore
type Result = runtimeratelimit.Result
type Store = runtimeratelimit.Store

const (
	HeaderLimit     = runtimeratelimit.HeaderLimit
	HeaderRemaining = runtimeratelimit.HeaderRemaining
	HeaderReset     = runtimeratelimit.HeaderReset
)

var DefaultErrorHandler = runtimeratelimit.DefaultErrorHandler
var DefaultLimitHandler = runtimeratelimit.DefaultLimitHandler
var KeyByRemoteAddr = runtimeratelimit.KeyByRemoteAddr
var New = runtimeratelimit.New
var NewInMemoryStore = runtimeratelimit.NewInMemoryStore
var NewRedisStore = runtimeratelimit.NewRedisStore
var WriteHeaders = runtimeratelimit.WriteHeaders
