package ssr

import runtimessr "github.com/cssbruno/gowdk/runtime/ssr"

type CondSpec = runtimessr.CondSpec
type ErrorHandler = runtimessr.ErrorHandler
type GuardFunc = runtimessr.GuardFunc
type GuardRegistry = runtimessr.GuardRegistry
type LayoutFunc = runtimessr.LayoutFunc
type LayoutRegistry = runtimessr.LayoutRegistry
type LayoutStack = runtimessr.LayoutStack
type ListField = runtimessr.ListField
type ListSpec = runtimessr.ListSpec
type LoadContext = runtimessr.LoadContext
type LoadFunc = runtimessr.LoadFunc
type RedirectError = runtimessr.RedirectError
type Route = runtimessr.Route
type Router = runtimessr.Router

var ComposeLayouts = runtimessr.ComposeLayouts
var DefaultErrorHandler = runtimessr.DefaultErrorHandler
var ElementPath = runtimessr.ElementPath
var IsNativeRBACGuard = runtimessr.IsNativeRBACGuard
var LoadPath = runtimessr.LoadPath
var NativeRBACGuard = runtimessr.NativeRBACGuard
var NewLoadContext = runtimessr.NewLoadContext
var Redirect = runtimessr.Redirect
var RedirectTarget = runtimessr.RedirectTarget
var RedirectTo = runtimessr.RedirectTo
var Register = runtimessr.Register
var RenderRegions = runtimessr.RenderRegions
var RunGuards = runtimessr.RunGuards
var RunGuardsWithAuth = runtimessr.RunGuardsWithAuth
