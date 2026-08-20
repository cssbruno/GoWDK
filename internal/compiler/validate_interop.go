package compiler

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/runtime/auth"
)

func validateInteropRegistrations(config gowdk.Config, ir gwdkir.Program) []ValidationError {
	type owner struct {
		id     string
		source string
		span   source.SourceSpan
	}
	var customOwner, nativeOwner *owner
	observe := func(guards []string, candidate owner) {
		for _, name := range guards {
			switch {
			case auth.IsPublicGuard(name):
			case name == "auth.required" && config.HasFeature(gowdk.FeatureAuth):
			case auth.IsNativeGuard(name):
				if nativeOwner == nil {
					copy := candidate
					nativeOwner = &copy
				}
			default:
				if customOwner == nil {
					copy := candidate
					customOwner = &copy
				}
			}
		}
	}
	for index := range ir.Pages {
		page := &ir.Pages[index]
		observe(page.Guards, owner{id: page.ID, source: page.Source, span: firstSpan(page.Blocks.Spans.Server, page.Blocks.Spans.View)})
	}
	for _, endpoint := range ir.Endpoints {
		candidate := owner{id: endpoint.PageID, source: endpoint.SourceFile, span: endpoint.Span}
		if page := pageByID(ir.Pages, endpoint.PageID); page != nil {
			candidate = owner{id: page.ID, source: page.Source, span: firstSpan(endpoint.Span, page.Blocks.Spans.Server, page.Blocks.Spans.View)}
		}
		observe(endpoint.Guards, candidate)
	}
	for _, subscription := range ir.RealtimeSubscriptions {
		observe(subscription.Guards, owner{id: subscription.OwnerID, source: subscription.Source, span: subscription.Span})
	}
	var diagnostics []ValidationError
	if customOwner != nil && !config.Interop.Guards.Configured() {
		diagnostics = append(diagnostics, ValidationError{
			Code: "missing_guard_registration", PageID: customOwner.id,
			Source: customOwner.source, Span: customOwner.span,
			Message: "custom guards require Config.Interop.Guards = gowdk.RegisterGuards(package.Guards)",
		})
	}
	if nativeOwner != nil && !config.HasFeature(gowdk.FeatureAuth) && !config.Interop.AuthProvider.Configured() {
		diagnostics = append(diagnostics, ValidationError{
			Code: "missing_auth_registration", PageID: nativeOwner.id,
			Source: nativeOwner.source, Span: nativeOwner.span,
			Message: "native role:/permission: guards require Config.Interop.AuthProvider = gowdk.RegisterAuthProvider(package.AuthProvider) when the auth addon is disabled",
		})
	}
	return diagnostics
}

func pageByID(pages []gwdkir.Page, id string) *gwdkir.Page {
	for index := range pages {
		if pages[index].ID == id {
			return &pages[index]
		}
	}
	return nil
}
