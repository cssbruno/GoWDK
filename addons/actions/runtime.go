package actions

import runtimeactions "github.com/cssbruno/gowdk/runtime/actions"

type CSRF = runtimeactions.CSRF
type CSRFOptions = runtimeactions.CSRFOptions
type CSRFTokenSource = runtimeactions.CSRFTokenSource
type CSRFValidator = runtimeactions.CSRFValidator
type Handler = runtimeactions.Handler
type Registry = runtimeactions.Registry

var NewCSRF = runtimeactions.NewCSRF
var DecodeForm = runtimeactions.DecodeForm
var ValidateRequired = runtimeactions.ValidateRequired
