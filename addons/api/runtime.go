package api

import (
	"net/http"

	runtimeapi "github.com/cssbruno/gowdk/runtime/api"
)

type ErrorBody = runtimeapi.ErrorBody
type ErrorInfo = runtimeapi.ErrorInfo
type Handler = runtimeapi.Handler
type JSONFieldDecoder = runtimeapi.JSONFieldDecoder
type Registry = runtimeapi.Registry
type StatusResult = runtimeapi.StatusResult

var ErrMultipleJSONValues = runtimeapi.ErrMultipleJSONValues
var ErrNilRequest = runtimeapi.ErrNilRequest
var ErrUnsupportedContentType = runtimeapi.ErrUnsupportedContentType
var Error = runtimeapi.Error
var JSON = runtimeapi.JSON
var NoContent = runtimeapi.NoContent
var NewJSONFieldDecoder = runtimeapi.NewJSONFieldDecoder
var QueryBool = runtimeapi.QueryBool
var QueryInt = runtimeapi.QueryInt
var QueryInt64 = runtimeapi.QueryInt64
var QueryString = runtimeapi.QueryString
var QueryStrings = runtimeapi.QueryStrings
var RequireJSONContentType = runtimeapi.RequireJSONContentType
var ResultStatus = runtimeapi.ResultStatus

func DecodeJSON[T any](request *http.Request) (T, error) {
	return runtimeapi.DecodeJSON[T](request)
}
