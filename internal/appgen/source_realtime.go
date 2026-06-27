package appgen

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

const (
	generatedRealtimeEventsPath       = "/_gowdk/realtime/events"
	generatedRealtimeQueryRefreshPath = "/_gowdk/realtime/query-refresh"
)

func generatedRealtimeEnabled(options Options) bool {
	return !options.ProxyBackend && (len(boundRealtimeSubscriptions(options)) > 0 || len(boundQueryInvalidations(options)) > 0)
}

func generatedRealtimeQueryInvalidationsEnabled(options Options) bool {
	return !options.ProxyBackend && len(boundQueryInvalidations(options)) > 0
}

func generatedRealtimeQueryRefreshEnabled(options Options) bool {
	return generatedRealtimeQueryInvalidationsEnabled(options) && len(ssrRegionRoutes(options.SSR)) > 0
}

func boundRealtimeSubscriptions(options Options) []gwdkir.RealtimeSubscription {
	if options.IR == nil || len(options.IR.RealtimeSubscriptions) == 0 {
		return nil
	}
	subscriptions := make([]gwdkir.RealtimeSubscription, 0, len(options.IR.RealtimeSubscriptions))
	for _, subscription := range options.IR.RealtimeSubscriptions {
		if subscription.Status != gwdkir.ContractBindingBound {
			continue
		}
		if strings.TrimSpace(subscription.EventType) == "" {
			continue
		}
		subscriptions = append(subscriptions, subscription)
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		left := realtimeEventEnvelopeType(subscriptions[i])
		right := realtimeEventEnvelopeType(subscriptions[j])
		if left != right {
			return left < right
		}
		if subscriptions[i].OwnerID != subscriptions[j].OwnerID {
			return subscriptions[i].OwnerID < subscriptions[j].OwnerID
		}
		return subscriptions[i].Query < subscriptions[j].Query
	})
	return subscriptions
}

func boundQueryInvalidations(options Options) []gwdkir.QueryInvalidation {
	if options.IR == nil || len(options.IR.QueryInvalidations) == 0 {
		return nil
	}
	invalidations := make([]gwdkir.QueryInvalidation, 0, len(options.IR.QueryInvalidations))
	for _, invalidation := range options.IR.QueryInvalidations {
		if invalidation.Status != gwdkir.ContractBindingBound {
			continue
		}
		if strings.TrimSpace(invalidation.EventType) == "" || strings.TrimSpace(invalidation.QueryType) == "" {
			continue
		}
		invalidations = append(invalidations, invalidation)
	}
	sort.Slice(invalidations, func(i, j int) bool {
		if invalidations[i].EventType != invalidations[j].EventType {
			return invalidations[i].EventType < invalidations[j].EventType
		}
		if invalidations[i].QueryType != invalidations[j].QueryType {
			return invalidations[i].QueryType < invalidations[j].QueryType
		}
		if invalidations[i].OwnerID != invalidations[j].OwnerID {
			return invalidations[i].OwnerID < invalidations[j].OwnerID
		}
		return invalidations[i].Query < invalidations[j].Query
	})
	return invalidations
}

type realtimeStreamRoute struct {
	Route    string
	Guards   []string
	Audience string
}

func generatedRealtimeStreamUsesGuards(options Options) bool {
	return len(realtimeStreamFallbackGuards(options)) > 0
}

func generatedRealtimeStreamUsesAudience(options Options) bool {
	return len(realtimeStreamRoutes(options)) > 0
}

func generatedRealtimeStreamUsesRouteMatching(options Options) bool {
	return generatedRealtimeStreamUsesGuards(options) && len(realtimeStreamRoutes(options)) > 0
}

func realtimeRoutesByPage(options Options) map[string]string {
	routesByPage := map[string]string{}
	if options.IR == nil {
		return routesByPage
	}
	for _, page := range options.IR.Pages {
		route := strings.TrimSpace(page.Route)
		if page.ID == "" || route == "" {
			continue
		}
		routesByPage[page.ID] = route
	}
	return routesByPage
}

func realtimeStreamRoutes(options Options) []realtimeStreamRoute {
	routesByPage := realtimeRoutesByPage(options)
	seen := map[string]bool{}
	var routes []realtimeStreamRoute
	for _, subscription := range boundRealtimeSubscriptions(options) {
		if subscription.OwnerKind != gwdkir.SourcePage {
			continue
		}
		route := routesByPage[subscription.OwnerID]
		if route == "" {
			continue
		}
		guards := runtimeGuardNames(subscription.Guards)
		key := route + "\x00" + strings.Join(guards, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		routes = append(routes, realtimeStreamRoute{Route: route, Guards: guards})
	}
	for _, invalidation := range boundQueryInvalidations(options) {
		if invalidation.OwnerKind != gwdkir.SourcePage {
			continue
		}
		route := routesByPage[invalidation.OwnerID]
		if route == "" {
			continue
		}
		guards := runtimeGuardNames(invalidation.Guards)
		key := route + "\x00" + strings.Join(guards, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		routes = append(routes, realtimeStreamRoute{Route: route, Guards: guards})
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Route != routes[j].Route {
			return routes[i].Route < routes[j].Route
		}
		return strings.Join(routes[i].Guards, "\x00") < strings.Join(routes[j].Guards, "\x00")
	})
	labels := map[string]string{}
	for index := range routes {
		label := labels[routes[index].Route]
		if label == "" {
			label = fmt.Sprintf("gowdk.route.%d", len(labels))
			labels[routes[index].Route] = label
		}
		routes[index].Audience = label
	}
	return routes
}

func realtimeRouteAudienceByRoute(options Options) map[string]string {
	audiences := map[string]string{}
	for _, route := range realtimeStreamRoutes(options) {
		if route.Route == "" || route.Audience == "" {
			continue
		}
		audiences[route.Route] = route.Audience
	}
	return audiences
}

func realtimeSubscriptionAudiences(options Options) map[string][]string {
	routesByPage := realtimeRoutesByPage(options)
	audiencesByRoute := realtimeRouteAudienceByRoute(options)
	audiences := map[string][]string{}
	for _, subscription := range boundRealtimeSubscriptions(options) {
		if subscription.OwnerKind != gwdkir.SourcePage {
			continue
		}
		eventType := realtimeEventEnvelopeType(subscription)
		route := routesByPage[subscription.OwnerID]
		audience := audiencesByRoute[route]
		if eventType == "" || audience == "" {
			continue
		}
		addRealtimeAudience(audiences, eventType, audience)
	}
	sortRealtimeAudienceMap(audiences)
	return audiences
}

func realtimeQueryAudiences(options Options) map[string][]string {
	routesByPage := realtimeRoutesByPage(options)
	audiencesByRoute := realtimeRouteAudienceByRoute(options)
	audiences := map[string][]string{}
	for _, invalidation := range boundQueryInvalidations(options) {
		if invalidation.OwnerKind != gwdkir.SourcePage {
			continue
		}
		queryType := strings.TrimSpace(invalidation.QueryType)
		route := routesByPage[invalidation.OwnerID]
		audience := audiencesByRoute[route]
		if queryType == "" || audience == "" {
			continue
		}
		addRealtimeAudience(audiences, queryType, audience)
	}
	sortRealtimeAudienceMap(audiences)
	return audiences
}

func addRealtimeAudience(audiences map[string][]string, key string, audience string) {
	for _, existing := range audiences[key] {
		if existing == audience {
			return
		}
	}
	audiences[key] = append(audiences[key], audience)
}

func sortRealtimeAudienceMap(audiences map[string][]string) {
	for key := range audiences {
		sort.Strings(audiences[key])
	}
}

func realtimeStreamFallbackGuards(options Options) []string {
	seen := map[string]bool{}
	var guards []string
	for _, subscription := range boundRealtimeSubscriptions(options) {
		for _, guard := range runtimeGuardNames(subscription.Guards) {
			if seen[guard] {
				continue
			}
			seen[guard] = true
			guards = append(guards, guard)
		}
	}
	for _, invalidation := range boundQueryInvalidations(options) {
		for _, guard := range runtimeGuardNames(invalidation.Guards) {
			if seen[guard] {
				continue
			}
			seen[guard] = true
			guards = append(guards, guard)
		}
	}
	sort.Strings(guards)
	return guards
}

func realtimeDecls(options Options) []ast.Decl {
	if !generatedRealtimeEnabled(options) {
		return nil
	}
	decls := []ast.Decl{
		realtimeEventsPathDecl(),
		realtimeFanoutMutexDecl(),
		realtimeFanoutVarDecl(options),
		realtimeSubscriptionEventTypesDecl(options),
		realtimeSubscriptionAudiencesDecl(options),
		realtimeQueryAudiencesDecl(options),
		realtimeQueryInvalidationsDecl(options),
		registerRealtimeFanoutDecl(),
		currentRealtimeFanoutDecl(),
		realtimeEventsHandlerDecl(options),
		realtimeAudienceScopedEventsDecl(),
		realtimeEventWithAudienceDecl(),
		realtimeQueryInvalidationAudienceEventsDecl(),
		realtimeSubscriptionFanoutTypeDecl(),
		realtimeSubscriptionFanoutSendDecl(),
	}
	if generatedRealtimeStreamUsesRouteMatching(options) || generatedRealtimeStreamUsesAudience(options) {
		decls = append(decls, realtimeStreamPathDecl())
	}
	if generatedRealtimeStreamUsesAudience(options) {
		decls = append(decls, realtimeStreamAudienceDecl(options))
	}
	if generatedRealtimeStreamUsesGuards(options) {
		decls = append(decls, realtimeStreamGuardsDecl(options))
	}
	return decls
}

func realtimeQueryRefreshPathDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.CONST, Specs: []ast.Spec{&ast.ValueSpec{
		Names:  []*ast.Ident{id("RealtimeQueryRefreshPath")},
		Values: []ast.Expr{stringLit(generatedRealtimeQueryRefreshPath)},
	}}}
}

func realtimeEventsPathDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.CONST, Specs: []ast.Spec{&ast.ValueSpec{
		Names:  []*ast.Ident{id("RealtimeEventsPath")},
		Values: []ast.Expr{stringLit(generatedRealtimeEventsPath)},
	}}}
}

func realtimeFanoutMutexDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("realtimeFanoutMu")},
		Type:  sel("sync", "RWMutex"),
	}}}
}

func realtimeFanoutVarDecl(options Options) ast.Decl {
	newSSE := call(sel("gowdkrealtime", "NewSSE"))
	if generatedRealtimeStreamUsesAudience(options) {
		newSSE = call(
			sel("gowdkrealtime", "NewSSE"),
			call(sel("gowdkrealtime", "WithSSEAudienceFromRequest"), id("realtimeStreamAudience")),
		)
	}
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names:  []*ast.Ident{id("realtimeFanout")},
		Type:   sel("gowdkrealtime", "PresentationFanout"),
		Values: []ast.Expr{newSSE},
	}}}
}

func realtimeSubscriptionEventTypesDecl(options Options) ast.Decl {
	seen := map[string]bool{}
	var eventTypes []string
	for _, subscription := range boundRealtimeSubscriptions(options) {
		eventType := realtimeEventEnvelopeType(subscription)
		if eventType == "" || seen[eventType] {
			continue
		}
		seen[eventType] = true
		eventTypes = append(eventTypes, eventType)
	}
	sort.Strings(eventTypes)
	elts := make([]ast.Expr, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		elts = append(elts, &ast.KeyValueExpr{Key: stringLit(eventType), Value: id("true")})
	}
	if generatedRealtimeQueryInvalidationsEnabled(options) {
		elts = append(elts, &ast.KeyValueExpr{Key: sel("gowdkcontracts", "QueryInvalidationPresentationEventType"), Value: id("true")})
	}
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("realtimeSubscriptionEventTypes")},
		Type:  &ast.MapType{Key: id("string"), Value: id("bool")},
		Values: []ast.Expr{&ast.CompositeLit{
			Type: &ast.MapType{Key: id("string"), Value: id("bool")},
			Elts: elts,
		}},
	}}}
}

func realtimeSubscriptionAudiencesDecl(options Options) ast.Decl {
	return stringSliceMapVarDecl("realtimeSubscriptionAudiences", realtimeSubscriptionAudiences(options))
}

func realtimeQueryAudiencesDecl(options Options) ast.Decl {
	return stringSliceMapVarDecl("realtimeQueryAudiences", realtimeQueryAudiences(options))
}

func stringSliceMapVarDecl(name string, values map[string][]string) ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names:  []*ast.Ident{id(name)},
		Type:   &ast.MapType{Key: id("string"), Value: &ast.ArrayType{Elt: id("string")}},
		Values: []ast.Expr{stringSliceMapExpr(values)},
	}}}
}

func stringSliceMapExpr(values map[string][]string) ast.Expr {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	elts := make([]ast.Expr, 0, len(keys))
	for _, key := range keys {
		elts = append(elts, &ast.KeyValueExpr{Key: stringLit(key), Value: stringSliceExpr(values[key])})
	}
	return &ast.CompositeLit{
		Type: &ast.MapType{Key: id("string"), Value: &ast.ArrayType{Elt: id("string")}},
		Elts: elts,
	}
}

func realtimeQueryInvalidationsDecl(options Options) ast.Decl {
	invalidations := boundQueryInvalidations(options)
	elts := make([]ast.Expr, 0, len(invalidations))
	for _, invalidation := range invalidations {
		elts = append(elts, &ast.CompositeLit{
			Type: sel("gowdkcontracts", "QueryInvalidation"),
			Elts: []ast.Expr{
				keyValue("EventCategory", realtimeEventCategoryExpr(invalidation.EventCategory)),
				keyValue("EventType", stringLit(invalidation.EventType)),
				keyValue("QueryType", stringLit(invalidation.QueryType)),
			},
		})
	}
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("realtimeQueryInvalidations")},
		Type:  &ast.ArrayType{Elt: sel("gowdkcontracts", "QueryInvalidation")},
		Values: []ast.Expr{&ast.CompositeLit{
			Type: &ast.ArrayType{Elt: sel("gowdkcontracts", "QueryInvalidation")},
			Elts: elts,
		}},
	}}}
}

func realtimeEventCategoryExpr(category string) ast.Expr {
	switch category {
	case "domain":
		return sel("gowdkcontracts", "DomainEvent")
	case "integration":
		return sel("gowdkcontracts", "IntegrationEvent")
	default:
		return sel("gowdkcontracts", "DomainEvent")
	}
}

func realtimeEventEnvelopeType(subscription gwdkir.RealtimeSubscription) string {
	eventType := strings.TrimSpace(subscription.EventType)
	if eventType == "" {
		return ""
	}
	importPath := strings.TrimSpace(subscription.EventImportPath)
	if importPath == "" {
		return eventType
	}
	return importPath + "." + eventType
}

func registerRealtimeFanoutDecl() ast.Decl {
	return funcDecl("RegisterRealtimeFanout", []*ast.Field{
		{Names: []*ast.Ident{id("fanout")}, Type: sel("gowdkrealtime", "PresentationFanout")},
	}, nil, []ast.Stmt{
		&ast.IfStmt{Cond: &ast.BinaryExpr{X: id("fanout"), Op: token.EQL, Y: id("nil")}, Body: block(&ast.ReturnStmt{})},
		exprStmt(call(selExpr(id("realtimeFanoutMu"), "Lock"))),
		&ast.DeferStmt{Call: call(selExpr(id("realtimeFanoutMu"), "Unlock"))},
		assign([]ast.Expr{id("realtimeFanout")}, id("fanout")),
	})
}

func currentRealtimeFanoutDecl() ast.Decl {
	return funcDecl("currentRealtimeFanout", nil, []*ast.Field{
		{Type: sel("gowdkrealtime", "PresentationFanout")},
	}, []ast.Stmt{
		exprStmt(call(selExpr(id("realtimeFanoutMu"), "RLock"))),
		&ast.DeferStmt{Call: call(selExpr(id("realtimeFanoutMu"), "RUnlock"))},
		&ast.ReturnStmt{Results: []ast.Expr{id("realtimeFanout")}},
	})
}

func realtimeEventsHandlerDecl(options Options) ast.Decl {
	stmts := []ast.Stmt{}
	if generatedRealtimeStreamUsesGuards(options) {
		stmts = append(stmts, &ast.IfStmt{
			Cond: &ast.UnaryExpr{
				Op: token.NOT,
				X:  call(sel("runGuards"), id("response"), id("request"), call(sel("realtimeStreamGuards"), id("request"))),
			},
			Body: block(&ast.ReturnStmt{}),
		})
	}
	stmts = append(stmts,
		&ast.IfStmt{
			Init: define([]ast.Expr{id("handler"), id("ok")}, &ast.TypeAssertExpr{
				X:    call(id("currentRealtimeFanout")),
				Type: sel("http", "Handler"),
			}),
			Cond: id("ok"),
			Body: block(
				exprStmt(call(selExpr(id("handler"), "ServeHTTP"), id("response"), id("request"))),
				&ast.ReturnStmt{},
			),
		},
		exprStmt(call(sel("http", "Error"), id("response"), stringLit("gowdk realtime fanout does not implement http.Handler"), sel("http", "StatusServiceUnavailable"))),
	)
	return funcDecl("realtimeEventsHandler", nil, []*ast.Field{
		{Type: sel("http", "Handler")},
	}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{call(sel("http", "HandlerFunc"), &ast.FuncLit{
			Type: &ast.FuncType{Params: &ast.FieldList{List: actionParams()}},
			Body: block(stmts...),
		})}},
	})
}

func realtimeStreamPathDecl() ast.Decl {
	stmts := []ast.Stmt{
		define([]ast.Expr{id("requestPath")}, call(
			selExpr(call(selExpr(selExpr(id("request"), "URL"), "Query")), "Get"),
			stringLit("path"),
		)),
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: id("requestPath"), Op: token.EQL, Y: stringLit("")},
			Body: block(&ast.IfStmt{
				Init: define([]ast.Expr{id("referer")}, call(selExpr(id("request"), "Referer"))),
				Cond: &ast.BinaryExpr{X: id("referer"), Op: token.NEQ, Y: stringLit("")},
				Body: block(&ast.IfStmt{
					Init: define([]ast.Expr{id("refererURL"), id("err")}, call(sel("neturl", "Parse"), id("referer"))),
					Cond: &ast.BinaryExpr{X: id("err"), Op: token.EQL, Y: id("nil")},
					Body: block(assign([]ast.Expr{id("requestPath")}, selExpr(id("refererURL"), "Path"))),
				}),
			}),
		},
		&ast.ReturnStmt{Results: []ast.Expr{id("requestPath")}},
	}
	return funcDecl("realtimeStreamPath", []*ast.Field{
		{Names: []*ast.Ident{id("request")}, Type: &ast.StarExpr{X: sel("http", "Request")}},
	}, []*ast.Field{{Type: id("string")}}, stmts)
}

func realtimeStreamAudienceDecl(options Options) ast.Decl {
	stmts := []ast.Stmt{
		define([]ast.Expr{id("requestPath")}, call(id("realtimeStreamPath"), id("request"))),
	}
	var routeStmts []ast.Stmt
	for _, route := range realtimeStreamRoutes(options) {
		routeStmts = append(routeStmts, &ast.IfStmt{
			Init: define([]ast.Expr{id("_"), id("ok")}, call(sel("gowdkroute", "Match"), stringLit(route.Route), id("requestPath"))),
			Cond: id("ok"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{stringSliceExpr([]string{route.Audience})}}),
		})
	}
	stmts = append(stmts,
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: id("requestPath"), Op: token.NEQ, Y: stringLit("")},
			Body: block(routeStmts...),
		},
		&ast.ReturnStmt{Results: []ast.Expr{id("nil")}},
	)
	return funcDecl("realtimeStreamAudience", []*ast.Field{
		{Names: []*ast.Ident{id("request")}, Type: &ast.StarExpr{X: sel("http", "Request")}},
	}, []*ast.Field{{Type: &ast.ArrayType{Elt: id("string")}}}, stmts)
}

func realtimeStreamGuardsDecl(options Options) ast.Decl {
	fallback := realtimeStreamFallbackGuards(options)
	stmts := []ast.Stmt{
		define([]ast.Expr{id("requestPath")}, call(id("realtimeStreamPath"), id("request"))),
	}
	var routeStmts []ast.Stmt
	for _, route := range realtimeStreamRoutes(options) {
		routeStmts = append(routeStmts, &ast.IfStmt{
			Init: define([]ast.Expr{id("_"), id("ok")}, call(sel("gowdkroute", "Match"), stringLit(route.Route), id("requestPath"))),
			Cond: id("ok"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{stringSliceExpr(route.Guards)}}),
		})
	}
	if len(routeStmts) > 0 {
		stmts = append(stmts,
			&ast.IfStmt{
				Cond: &ast.BinaryExpr{X: id("requestPath"), Op: token.NEQ, Y: stringLit("")},
				Body: block(routeStmts...),
			},
		)
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{stringSliceExpr(fallback)}})
	return funcDecl("realtimeStreamGuards", []*ast.Field{
		{Names: []*ast.Ident{id("request")}, Type: &ast.StarExpr{X: sel("http", "Request")}},
	}, []*ast.Field{{Type: &ast.ArrayType{Elt: id("string")}}}, stmts)
}

func eventEnvelopeSliceType() ast.Expr {
	return &ast.ArrayType{Elt: sel("gowdkcontracts", "EventEnvelope")}
}

func eventEnvelopeSliceExpr(values ...ast.Expr) ast.Expr {
	return &ast.CompositeLit{
		Type: eventEnvelopeSliceType(),
		Elts: values,
	}
}

func realtimeAudienceScopedEventsDecl() ast.Decl {
	return funcDecl("realtimeAudienceScopedEvents", []*ast.Field{
		{Names: []*ast.Ident{id("event")}, Type: sel("gowdkcontracts", "EventEnvelope")},
	}, []*ast.Field{{Type: eventEnvelopeSliceType()}}, []ast.Stmt{
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: selExpr(id("event"), "Category"), Op: token.NEQ, Y: sel("gowdkcontracts", "PresentationEvent")},
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil")}}),
		},
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: selExpr(id("event"), "Type"), Op: token.EQL, Y: sel("gowdkcontracts", "QueryInvalidationPresentationEventType")},
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{call(id("realtimeQueryInvalidationAudienceEvents"), id("event"))}}),
		},
		&ast.IfStmt{
			Cond: &ast.UnaryExpr{Op: token.NOT, X: &ast.IndexExpr{X: id("realtimeSubscriptionEventTypes"), Index: selExpr(id("event"), "Type")}},
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil")}}),
		},
		define([]ast.Expr{id("audiences")}, &ast.IndexExpr{X: id("realtimeSubscriptionAudiences"), Index: selExpr(id("event"), "Type")}),
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: call(id("len"), id("audiences")), Op: token.EQL, Y: intLit(0)},
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{eventEnvelopeSliceExpr(id("event"))}}),
		},
		define([]ast.Expr{id("scoped")}, call(id("make"), eventEnvelopeSliceType(), intLit(0), call(id("len"), id("audiences")))),
		&ast.RangeStmt{
			Key:   id("_"),
			Value: id("audience"),
			Tok:   token.DEFINE,
			X:     id("audiences"),
			Body:  block(assign([]ast.Expr{id("scoped")}, call(id("append"), id("scoped"), call(id("realtimeEventWithAudience"), id("event"), id("audience"))))),
		},
		&ast.ReturnStmt{Results: []ast.Expr{id("scoped")}},
	})
}

func realtimeEventWithAudienceDecl() ast.Decl {
	return funcDecl("realtimeEventWithAudience", []*ast.Field{
		{Names: []*ast.Ident{id("event")}, Type: sel("gowdkcontracts", "EventEnvelope")},
		{Names: []*ast.Ident{id("audience")}, Type: id("string")},
	}, []*ast.Field{{Type: sel("gowdkcontracts", "EventEnvelope")}}, []ast.Stmt{
		define([]ast.Expr{id("scoped")}, id("event")),
		define([]ast.Expr{id("audiences")}, &ast.CallExpr{
			Fun:      id("append"),
			Args:     []ast.Expr{&ast.CompositeLit{Type: &ast.ArrayType{Elt: id("string")}}, selExpr(id("event"), "Audience")},
			Ellipsis: token.Pos(1),
		}),
		assign([]ast.Expr{selExpr(id("scoped"), "Audience")}, call(id("append"), id("audiences"), id("audience"))),
		&ast.ReturnStmt{Results: []ast.Expr{id("scoped")}},
	})
}

func realtimeQueryInvalidationAudienceEventsDecl() ast.Decl {
	return funcDecl("realtimeQueryInvalidationAudienceEvents", []*ast.Field{
		{Names: []*ast.Ident{id("event")}, Type: sel("gowdkcontracts", "EventEnvelope")},
	}, []*ast.Field{{Type: eventEnvelopeSliceType()}}, []ast.Stmt{
		define([]ast.Expr{id("notice"), id("ok")}, &ast.TypeAssertExpr{X: selExpr(id("event"), "Value"), Type: sel("gowdkcontracts", "QueryInvalidationNotice")}),
		&ast.IfStmt{
			Cond: &ast.UnaryExpr{Op: token.NOT, X: id("ok")},
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{eventEnvelopeSliceExpr(id("event"))}}),
		},
		define([]ast.Expr{id("queriesByAudience")}, &ast.CompositeLit{Type: &ast.MapType{Key: id("string"), Value: &ast.ArrayType{Elt: id("string")}}}),
		define([]ast.Expr{id("audiences")}, &ast.CompositeLit{Type: &ast.ArrayType{Elt: id("string")}}),
		define([]ast.Expr{id("seenAudience")}, &ast.CompositeLit{Type: &ast.MapType{Key: id("string"), Value: id("bool")}}),
		&ast.RangeStmt{
			Key:   id("_"),
			Value: id("query"),
			Tok:   token.DEFINE,
			X:     selExpr(id("notice"), "Queries"),
			Body: block(&ast.RangeStmt{
				Key:   id("_"),
				Value: id("audience"),
				Tok:   token.DEFINE,
				X:     &ast.IndexExpr{X: id("realtimeQueryAudiences"), Index: id("query")},
				Body: block(
					&ast.IfStmt{
						Cond: &ast.UnaryExpr{Op: token.NOT, X: &ast.IndexExpr{X: id("seenAudience"), Index: id("audience")}},
						Body: block(
							assign([]ast.Expr{&ast.IndexExpr{X: id("seenAudience"), Index: id("audience")}}, id("true")),
							assign([]ast.Expr{id("audiences")}, call(id("append"), id("audiences"), id("audience"))),
						),
					},
					assign([]ast.Expr{&ast.IndexExpr{X: id("queriesByAudience"), Index: id("audience")}}, call(id("append"), &ast.IndexExpr{X: id("queriesByAudience"), Index: id("audience")}, id("query"))),
				),
			}),
		},
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: call(id("len"), id("audiences")), Op: token.EQL, Y: intLit(0)},
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{eventEnvelopeSliceExpr(id("event"))}}),
		},
		define([]ast.Expr{id("scoped")}, call(id("make"), eventEnvelopeSliceType(), intLit(0), call(id("len"), id("audiences")))),
		&ast.RangeStmt{
			Key:   id("_"),
			Value: id("audience"),
			Tok:   token.DEFINE,
			X:     id("audiences"),
			Body: block(
				define([]ast.Expr{id("scopedNotice")}, id("notice")),
				assign([]ast.Expr{selExpr(id("scopedNotice"), "Queries")}, &ast.IndexExpr{X: id("queriesByAudience"), Index: id("audience")}),
				define([]ast.Expr{id("scopedEvent")}, call(id("realtimeEventWithAudience"), id("event"), id("audience"))),
				assign([]ast.Expr{selExpr(id("scopedEvent"), "Value")}, id("scopedNotice")),
				assign([]ast.Expr{id("scoped")}, call(id("append"), id("scoped"), id("scopedEvent"))),
			),
		},
		&ast.ReturnStmt{Results: []ast.Expr{id("scoped")}},
	})
}

func realtimeSubscriptionFanoutTypeDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.TypeSpec{
		Name: id("realtimeSubscriptionFanout"),
		Type: &ast.StructType{Fields: &ast.FieldList{List: []*ast.Field{
			{Names: []*ast.Ident{id("inner")}, Type: sel("gowdkrealtime", "PresentationFanout")},
		}}},
	}}}
}

func realtimeSubscriptionFanoutSendDecl() ast.Decl {
	return &ast.FuncDecl{
		Recv: &ast.FieldList{List: []*ast.Field{{
			Names: []*ast.Ident{id("fanout")},
			Type:  id("realtimeSubscriptionFanout"),
		}}},
		Name: id("SendPresentationEvents"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{List: []*ast.Field{
				{Names: []*ast.Ident{id("ctx")}, Type: sel("context", "Context")},
				{Names: []*ast.Ident{id("events")}, Type: &ast.ArrayType{Elt: sel("gowdkcontracts", "EventEnvelope")}},
			}},
			Results: &ast.FieldList{List: []*ast.Field{{Type: id("error")}}},
		},
		Body: block(
			&ast.IfStmt{
				Cond: &ast.BinaryExpr{X: selExpr(id("fanout"), "inner"), Op: token.EQL, Y: id("nil")},
				Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil")}}),
			},
			define([]ast.Expr{id("filtered")}, &ast.CompositeLit{Type: &ast.ArrayType{Elt: sel("gowdkcontracts", "EventEnvelope")}}),
			&ast.RangeStmt{
				Key:   id("_"),
				Value: id("event"),
				Tok:   token.DEFINE,
				X:     id("events"),
				Body: block(assign([]ast.Expr{id("filtered")}, &ast.CallExpr{
					Fun:      id("append"),
					Args:     []ast.Expr{id("filtered"), call(id("realtimeAudienceScopedEvents"), id("event"))},
					Ellipsis: token.Pos(1),
				})),
			},
			&ast.IfStmt{
				Cond: &ast.BinaryExpr{X: call(id("len"), id("filtered")), Op: token.EQL, Y: intLit(0)},
				Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil")}}),
			},
			&ast.ReturnStmt{Results: []ast.Expr{call(selExpr(selExpr(id("fanout"), "inner"), "SendPresentationEvents"), id("ctx"), id("filtered"))}},
		),
	}
}
