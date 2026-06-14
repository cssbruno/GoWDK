package appgen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

func actionHandlerSource(actions []ActionEndpoint, csrf bool) (string, error) {
	sorted := backendAdapterIR(Options{Actions: actions}).Actions
	decls := []ast.Decl{actionFuncDecl(sorted, csrf, false)}
	if len(sorted) > 0 {
		decls = append(decls, actionRequestPathDecl())
		decls = append(decls, actionDecoderDecls(sorted)...)
	}
	return printActionDecls(decls)
}

func printActionDecls(decls []ast.Decl) (string, error) {
	var buffer bytes.Buffer
	fileSet := token.NewFileSet()
	for index, decl := range decls {
		if index > 0 {
			_, _ = buffer.Write([]byte("\n\n"))
		}
		if err := printer.Fprint(&buffer, fileSet, decl); err != nil {
			return "", fmt.Errorf("print generated declaration: %w", err)
		}
	}
	return formatGoDeclSnippet(buffer.String())
}

func actionsUseValidation(actions []BackendActionAdapter) bool {
	for _, action := range actions {
		if actionsUseActionValidation(action) {
			return true
		}
	}
	return false
}

func actionsUseLengthValidation(actions []BackendActionAdapter) bool {
	for _, action := range actions {
		if !actionsUseActionValidation(action) {
			continue
		}
		for _, rule := range action.ValidationRules {
			if rule.MinLength > 0 || rule.MaxLength > 0 {
				return true
			}
		}
	}
	return false
}

func actionsUseActionValidation(action BackendActionAdapter) bool {
	return action.Binding.Status != source.BackendBindingMissing && action.Binding.Status != source.BackendBindingUnsupportedSignature && action.ValidatesInput
}

func actionsUseForm(actions []BackendActionAdapter) bool {
	for _, action := range actions {
		if action.Binding.Status != source.BackendBindingMissing && action.Binding.Status != source.BackendBindingUnsupportedSignature && actionNeedsValues(action) {
			return true
		}
	}
	return false
}

func actionsUseFragments(actions []BackendActionAdapter) bool {
	for _, action := range actions {
		if actionUsesPartialAddon(action) {
			return true
		}
	}
	return false
}

func actionUsesPartialAddon(action BackendActionAdapter) bool {
	return action.Binding.Status == "" && len(action.Fragments) > 0
}

func actionsParseForm(actions []BackendActionAdapter) bool {
	for _, action := range actions {
		if action.Binding.Status != source.BackendBindingMissing && action.Binding.Status != source.BackendBindingUnsupportedSignature {
			return true
		}
	}
	return false
}

func sortedActionEndpoints(actions []ActionEndpoint) []ActionEndpoint {
	sorted := append([]ActionEndpoint(nil), actions...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Route == sorted[j].Route {
			return sorted[i].ActionName < sorted[j].ActionName
		}
		return sorted[i].Route < sorted[j].Route
	})
	return sorted
}

func sortedActionAdapters(actions []BackendActionAdapter) []BackendActionAdapter {
	sorted := append([]BackendActionAdapter(nil), actions...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Route == sorted[j].Route {
			return sorted[i].ActionName < sorted[j].ActionName
		}
		return sorted[i].Route < sorted[j].Route
	})
	return sorted
}

func actionNeedsValues(action BackendActionAdapter) bool {
	if action.Binding.Status != source.BackendBindingBound {
		return true
	}
	if action.ValidatesInput {
		return true
	}
	return action.Binding.Signature != source.BackendSignatureAction0
}

func actionFuncDecl(actions []BackendActionAdapter, csrf bool, rateLimit bool) *ast.FuncDecl {
	if len(actions) == 0 {
		return funcDecl("action", actionParams(), boolResults(), []ast.Stmt{returnBool(false)})
	}
	results := boolResults()
	if actionsUseErrorPages(actions) {
		results = namedBoolResults("handled")
	}
	var clauses []ast.Stmt
	for _, action := range actions {
		clauses = append(clauses, &ast.CaseClause{
			List: []ast.Expr{stringLit(cleanRoutePath(action.Route))},
			Body: actionCaseStmts(action, csrf, rateLimit),
		})
	}
	clauses = append(clauses, &ast.CaseClause{Body: []ast.Stmt{returnBool(false)}})
	return funcDecl("action", actionParams(), results, []ast.Stmt{
		define([]ast.Expr{id("requestPath")}, call(sel("actionRequestPath"), selExpr(selExpr(id("request"), "URL"), "Path"))),
		&ast.SwitchStmt{
			Tag:  id("requestPath"),
			Body: &ast.BlockStmt{List: clauses},
		},
	})
}

func actionRequestPathDecl() *ast.FuncDecl {
	return funcDecl("actionRequestPath", []*ast.Field{
		{Names: []*ast.Ident{id("value")}, Type: id("string")},
	}, []*ast.Field{{Type: id("string")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{
			call(sel("path", "Clean"), &ast.BinaryExpr{X: stringLit("/"), Op: token.ADD, Y: id("value")}),
		}},
	})
}

func actionCaseStmts(action BackendActionAdapter, csrf bool, rateLimit bool) []ast.Stmt {
	stmts := endpointContextStmts("action", action.PageID, action.ActionName, actionAdapterMethod(action), action.Route, action.ErrorPage)
	if action.ErrorPage != "" {
		stmts = append(stmts, endpointPanicBoundaryStmt())
	}
	stmts = append(stmts, rateLimitStmts(rateLimit)...)
	stmts = append(stmts, guardStmts(action.Guards)...)
	if action.Binding.Status != "" && action.Binding.Status != source.BackendBindingBound {
		stmts = append(stmts, backendNotImplementedStmts(action.Binding, "action")...)
		stmts = append(stmts, returnBool(true))
		return stmts
	}
	stmts = append(stmts, actionParseFormStmts(csrf)...)
	if actionNeedsValues(action) {
		stmts = append(stmts, define([]ast.Expr{id("values")}, call(sel("gowdkform", "FromURLValues"), selExpr(id("request"), "PostForm"))))
	}
	stmts = append(stmts, actionInputDecodeStmts(action)...)
	if action.Binding.Status == source.BackendBindingBound {
		stmts = append(stmts, boundActionResultStmts(action)...)
	} else {
		stmts = append(stmts, actionPartialBranchStmts(action)...)
		stmts = append(stmts, actionResultStmts(action)...)
	}
	stmts = append(stmts, returnBool(true))
	return stmts
}

func actionParseFormStmts(csrf bool) []ast.Stmt {
	return actionParseFormStmtsWithErrors(csrf, false)
}

func contractParseFormStmts(csrf bool) []ast.Stmt {
	return actionParseFormStmtsWithErrors(csrf, true)
}

func actionParseFormStmtsWithErrors(csrf bool, jsonErrors bool) []ast.Stmt {
	writeError := writeNoStoreErrorStmt
	if jsonErrors {
		writeError = writeNoStoreJSONErrorStmt
	}
	stmts := []ast.Stmt{
		assign([]ast.Expr{selExpr(id("request"), "Body")}, call(sel("http", "MaxBytesReader"), id("response"), selExpr(id("request"), "Body"), id("maxActionBodyBytes"))),
		&ast.IfStmt{
			Init: define([]ast.Expr{id("err")}, call(selExpr(id("request"), "ParseForm"))),
			Cond: notNil("err"),
			Body: block(
				&ast.IfStmt{
					Cond: call(sel("strings", "Contains"), call(selExpr(id("err"), "Error")), stringLit("request body too large")),
					Body: block(
						writeError(sel("http", "StatusRequestEntityTooLarge"), "request body too large"),
						returnBool(true),
					),
				},
				writeError(sel("http", "StatusBadRequest"), "invalid form"),
				returnBool(true),
			),
		},
	}
	if csrf {
		stmts = append(stmts, &ast.IfStmt{
			Cond: notNil("csrfValidator"),
			Body: block(&ast.IfStmt{
				Init: define([]ast.Expr{id("err")}, call(selExpr(id("csrfValidator"), "Validate"), id("request"))),
				Cond: notNil("err"),
				Body: block(
					writeError(sel("http", "StatusForbidden"), "invalid csrf token"),
					returnBool(true),
				),
			}),
		})
	}
	return stmts
}

func actionInputDecodeStmts(action BackendActionAdapter) []ast.Stmt {
	if action.Binding.Status == source.BackendBindingBound {
		return boundActionInputDecodeStmts(action)
	}
	if action.InputType == "" {
		return expectedValuesStmts(action)
	}
	stmts := []ast.Stmt{
		define([]ast.Expr{id("input"), id("err")}, call(sel(actionDecoderName(action)), id("values"))),
		ifErrReturnInvalidForm(),
		assign([]ast.Expr{id("_")}, id("input")),
		assign([]ast.Expr{id("values")}, selExpr(id("input"), "Values")),
	}
	if action.ValidatesInput {
		stmts = append(stmts, actionRequiredValidationStmts(action)...)
	}
	return stmts
}

func boundActionInputDecodeStmts(action BackendActionAdapter) []ast.Stmt {
	switch action.Binding.Signature {
	case source.BackendSignatureAction0:
		if action.ValidatesInput {
			return actionRequiredValidationStmts(action)
		}
		return nil
	case source.BackendSignatureActionValues:
		stmts := expectedValuesStmts(action)
		if action.ValidatesInput {
			stmts = append(stmts, actionRequiredValidationStmts(action)...)
		}
		return stmts
	case source.BackendSignatureActionForm, source.BackendSignatureActionFormPtr:
		stmts := expectedValuesStmts(action)
		stmts = append(stmts,
			define([]ast.Expr{id("input"), id("err")}, call(sel(boundActionDecoderName(action)), id("values"))),
			ifErrReturnInvalidForm(),
		)
		if action.ValidatesInput {
			stmts = append(stmts, actionRequiredValidationStmts(action)...)
		}
		return stmts
	default:
		return expectedValuesStmts(action)
	}
}

func expectedValuesStmts(action BackendActionAdapter) []ast.Stmt {
	return []ast.Stmt{&ast.IfStmt{
		Init: define([]ast.Expr{id("decodedValues"), id("err")}, call(sel("gowdkform", "DecodeExpected"), id("values"), formSchemaExpr(action.InputFields))),
		Cond: notNil("err"),
		Body: block(
			writeNoStoreErrorStmt(sel("http", "StatusBadRequest"), "invalid form"),
			returnBool(true),
		),
		Else: block(assign([]ast.Expr{id("values")}, id("decodedValues"))),
	}}
}

func actionRequiredValidationStmts(action BackendActionAdapter) []ast.Stmt {
	stmts := []ast.Stmt{
		define([]ast.Expr{id("validation")}, &ast.CompositeLit{Type: sel("gowdkvalidation", "Result")}),
	}
	for _, field := range action.RequiredFields {
		stmts = append(stmts, &ast.IfStmt{
			Cond: &ast.UnaryExpr{Op: token.NOT, X: call(selExpr(id("values"), "HasSubmitted"), stringLit(field))},
			Body: block(exprStmt(call(selExpr(id("validation"), "Add"), stringLit(field), stringLit(actionValidationMessage(action.RequiredMessages[field], "required"))))),
		})
	}
	for _, rule := range action.ValidationRules {
		stmts = append(stmts, actionValidationRuleStmt(rule))
	}
	stmts = append(stmts,
		&ast.IfStmt{
			Cond: &ast.UnaryExpr{Op: token.NOT, X: call(selExpr(id("validation"), "OK"))},
			Body: block(
				define([]ast.Expr{id("partial")}, trimHeaderCall("X-GOWDK-Partial")),
				define([]ast.Expr{id("validationTarget")}, trimHeaderCall("X-GOWDK-Target")),
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X: &ast.BinaryExpr{
								X:  id("partial"),
								Op: token.NEQ,
								Y:  stringLit(""),
							},
							Op: token.LAND,
							Y: &ast.BinaryExpr{
								X:  id("partial"),
								Op: token.NEQ,
								Y:  stringLit("0"),
							},
						},
						Op: token.LAND,
						Y: &ast.BinaryExpr{
							X:  id("validationTarget"),
							Op: token.NEQ,
							Y:  stringLit(""),
						},
					},
					Body: block(
						writeNoStoreHTTPStmt(call(sel("gowdkresponse", "ValidationFragment"), id("validationTarget"), id("validation"))),
						returnBool(true),
					),
				},
				writeNoStoreErrorStmt(sel("http", "StatusUnprocessableEntity"), "validation failed"),
				returnBool(true),
			),
		},
	)
	return stmts
}

func actionValidationRuleStmt(rule ActionValidationRule) ast.Stmt {
	var checks []ast.Stmt
	if rule.MinLength > 0 {
		checks = append(checks, &ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X:  call(sel("utf8", "RuneCountInString"), id("value")),
				Op: token.LSS,
				Y:  intLit(rule.MinLength),
			},
			Body: block(exprStmt(call(selExpr(id("validation"), "Add"), stringLit(rule.Field), stringLit(actionValidationMessage(rule.MinLengthMessage, "minlength"))))),
		})
	}
	if rule.MaxLength > 0 {
		checks = append(checks, &ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X:  call(sel("utf8", "RuneCountInString"), id("value")),
				Op: token.GTR,
				Y:  intLit(rule.MaxLength),
			},
			Body: block(exprStmt(call(selExpr(id("validation"), "Add"), stringLit(rule.Field), stringLit(actionValidationMessage(rule.MaxLengthMessage, "maxlength"))))),
		})
	}
	if rule.Pattern != "" {
		checks = append(checks, &ast.IfStmt{
			Init: define([]ast.Expr{id("matched"), id("err")}, call(sel("gowdkvalidation", "MatchPattern"), stringLit(rule.Pattern), id("value"))),
			Cond: &ast.BinaryExpr{
				X:  notNil("err"),
				Op: token.LOR,
				Y:  &ast.UnaryExpr{Op: token.NOT, X: id("matched")},
			},
			Body: block(exprStmt(call(selExpr(id("validation"), "Add"), stringLit(rule.Field), stringLit(actionValidationMessage(rule.PatternMessage, "pattern"))))),
		})
	}
	return block(
		define([]ast.Expr{id("value")}, call(sel("strings", "TrimSpace"), call(selExpr(id("values"), "First"), stringLit(rule.Field)))),
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: id("value"), Op: token.NEQ, Y: stringLit("")},
			Body: block(checks...),
		},
	)
}

func actionValidationMessage(custom string, fallback string) string {
	custom = strings.TrimSpace(custom)
	if custom == "" {
		return fallback
	}
	return custom
}

func boundActionResultStmts(action BackendActionAdapter) []ast.Stmt {
	args := []ast.Expr{id("ctx")}
	switch action.Binding.Signature {
	case source.BackendSignatureAction0:
	case source.BackendSignatureActionForm:
		args = append(args, id("input"))
	case source.BackendSignatureActionFormPtr:
		args = append(args, &ast.UnaryExpr{Op: token.AND, X: id("input")})
	default:
		args = append(args, id("values"))
	}
	return []ast.Stmt{
		define([]ast.Expr{id("result"), id("err")}, call(sel(action.BackendAlias, action.Binding.FunctionName), args...)),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(
				writeNoStoreHandlerErrorExprStmt(id("err"), sel("http", "StatusInternalServerError")),
				returnBool(true),
			),
		},
		assign([]ast.Expr{id("_")}, call(sel("gowdkresponse", "WriteNoStoreHTTP"), id("response"), id("result"))),
	}
}

func actionMethod(action ActionEndpoint) string {
	method := strings.ToUpper(strings.TrimSpace(action.Method))
	if method == "" {
		return "POST"
	}
	return method
}

func actionAdapterMethod(action BackendActionAdapter) string {
	method := strings.ToUpper(strings.TrimSpace(action.Method))
	if method == "" {
		return "POST"
	}
	return method
}

func endpointContextStmt(kind, pageID, name, method, route, errorPage string) ast.Stmt {
	return define(
		[]ast.Expr{id("ctx")},
		call(
			sel("gowdkruntime", "WithEndpoint"),
			call(sel("gowdkruntime", "WithRequest"), call(selExpr(id("request"), "Context")), id("request")),
			endpointMetadataExpr(kind, pageID, name, method, route, errorPage),
		),
	)
}

func endpointContextStmts(kind, pageID, name, method, route, errorPage string) []ast.Stmt {
	return []ast.Stmt{
		endpointContextStmt(kind, pageID, name, method, route, errorPage),
		assign([]ast.Expr{id("request")}, call(selExpr(id("request"), "WithContext"), id("ctx"))),
	}
}

func endpointMetadataExpr(kind, pageID, name, method, route, errorPage string) ast.Expr {
	elts := []ast.Expr{
		keyValue("Kind", stringLit(kind)),
		keyValue("PageID", stringLit(pageID)),
		keyValue("Name", stringLit(name)),
		keyValue("Method", stringLit(method)),
		keyValue("Path", stringLit(route)),
	}
	if errorPage != "" {
		elts = append(elts, keyValue("ErrorPage", stringLit(errorPage)))
	}
	return &ast.CompositeLit{
		Type: sel("gowdkruntime", "EndpointMetadata"),
		Elts: elts,
	}
}

func actionsUseErrorPages(actions []BackendActionAdapter) bool {
	for _, action := range actions {
		if action.ErrorPage != "" {
			return true
		}
	}
	return false
}

func endpointPanicBoundaryStmt() ast.Stmt {
	return &ast.DeferStmt{Call: call(&ast.FuncLit{
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: block(&ast.IfStmt{
			Init: define([]ast.Expr{id("recovered")}, call(id("recover"))),
			Cond: notNil("recovered"),
			Body: block(
				assign([]ast.Expr{id("handled")}, id("true")),
				exprStmt(call(sel("gowdkruntime", "RecoverEndpointPanic"), id("response"), id("request"), id("recovered"))),
			),
		}),
	})}
}

func actionPartialBranchStmts(action BackendActionAdapter) []ast.Stmt {
	body := []ast.Stmt{}
	if len(action.Fragments) == 0 {
		body = append(body,
			writeNoStoreErrorStmt(sel("http", "StatusBadRequest"), "partial fragment not found"),
			returnBool(true),
		)
	} else {
		body = append(body, define([]ast.Expr{id("target")}, trimHeaderCall("X-GOWDK-Target")))
		body = append(body, fragmentSwitchStmt(action.Fragments))
	}
	return []ast.Stmt{
		define([]ast.Expr{id("partial")}, trimHeaderCall("X-GOWDK-Partial")),
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X:  &ast.BinaryExpr{X: id("partial"), Op: token.NEQ, Y: stringLit("")},
				Op: token.LAND,
				Y:  &ast.BinaryExpr{X: id("partial"), Op: token.NEQ, Y: stringLit("0")},
			},
			Body: &ast.BlockStmt{List: body},
		},
	}
}

func fragmentSwitchStmt(fragments []ActionFragment) ast.Stmt {
	clauses := make([]ast.Stmt, 0, len(fragments)+1)
	for index, fragment := range fragments {
		var list []ast.Expr
		if index == 0 {
			list = append(list, stringLit(""))
		}
		list = append(list, stringLit(fragment.Target))
		clauses = append(clauses, &ast.CaseClause{
			List: list,
			Body: fragmentResponseStmts(fragment),
		})
	}
	clauses = append(clauses, &ast.CaseClause{Body: []ast.Stmt{
		writeNoStoreErrorStmt(sel("http", "StatusNotFound"), "partial fragment not found"),
		returnBool(true),
	}})
	return &ast.SwitchStmt{Tag: id("target"), Body: &ast.BlockStmt{List: clauses}}
}

func fragmentResponseStmts(fragment ActionFragment) []ast.Stmt {
	return []ast.Stmt{
		define([]ast.Expr{id("fragment")}, call(sel("gowdkpartial", "Fragment"), stringLit(fragment.Target), stringLit(fragment.HTML))),
		&ast.IfStmt{
			Init: define([]ast.Expr{id("swap")}, trimHeaderCall("X-GOWDK-Swap")),
			Cond: &ast.BinaryExpr{X: id("swap"), Op: token.NEQ, Y: stringLit("")},
			Body: block(&ast.IfStmt{
				Init: define([]ast.Expr{id("swapped"), id("err")}, call(
					sel("gowdkpartial", "Swap"),
					selExpr(id("fragment"), "Target"),
					call(sel("gowdkpartial", "SwapMode"), id("swap")),
					selExpr(id("fragment"), "Body"),
				)),
				Cond: &ast.BinaryExpr{X: id("err"), Op: token.EQL, Y: id("nil")},
				Body: block(assign([]ast.Expr{id("fragment")}, id("swapped"))),
			}),
		},
		assign([]ast.Expr{id("_")}, call(sel("gowdkresponse", "WriteNoStoreHTTP"), id("response"), id("fragment"))),
		returnBool(true),
	}
}

func actionResultStmts(action BackendActionAdapter) []ast.Stmt {
	if strings.TrimSpace(action.Redirect) == "" {
		return []ast.Stmt{
			setNoStoreHeaderStmt(),
			exprStmt(call(selExpr(id("response"), "WriteHeader"), sel("http", "StatusNoContent"))),
		}
	}
	return []ast.Stmt{writeNoStoreHTTPStmt(call(sel("gowdkresponse", "RedirectTo"), stringLit(action.Redirect)))}
}

func actionDecoderDecls(actions []BackendActionAdapter) []ast.Decl {
	var decls []ast.Decl
	for _, inputType := range uniqueInputTypes(actions) {
		decls = append(decls, &ast.GenDecl{
			Tok: token.TYPE,
			Specs: []ast.Spec{&ast.TypeSpec{
				Name: id(inputType),
				Type: &ast.StructType{Fields: &ast.FieldList{List: []*ast.Field{{
					Names: []*ast.Ident{id("Values")},
					Type:  sel("gowdkform", "Values"),
				}}}},
			}},
		})
	}
	for _, action := range actions {
		if action.Binding.Status == source.BackendBindingBound || action.Binding.Status == source.BackendBindingMissing || action.Binding.Status == source.BackendBindingUnsupportedSignature || action.InputType == "" {
			continue
		}
		decls = append(decls, valuesActionDecoderDecl(action))
	}
	for _, action := range actions {
		if !actionUsesBoundInputDecoder(action) {
			continue
		}
		decls = append(decls, boundActionDecoderDecl(action))
	}
	return decls
}

func valuesActionDecoderDecl(action BackendActionAdapter) *ast.FuncDecl {
	inputType := action.InputType
	return funcDecl(actionDecoderName(action), []*ast.Field{
		{Names: []*ast.Ident{id("values")}, Type: sel("gowdkform", "Values")},
	}, []*ast.Field{{Type: id(inputType)}, {Type: id("error")}}, []ast.Stmt{
		define([]ast.Expr{id("decoded"), id("err")}, call(sel("gowdkform", "DecodeExpected"), id("values"), formSchemaExpr(action.InputFields))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{
				&ast.CompositeLit{Type: id(inputType)},
				id("err"),
			}}),
		},
		&ast.ReturnStmt{Results: []ast.Expr{
			&ast.CompositeLit{
				Type: id(inputType),
				Elts: []ast.Expr{keyValue("Values", id("decoded"))},
			},
			id("nil"),
		}},
	})
}

func uniqueInputTypes(actions []BackendActionAdapter) []string {
	seen := map[string]bool{}
	var types []string
	for _, action := range actions {
		if action.Binding.Status == source.BackendBindingBound || action.Binding.Status == source.BackendBindingMissing || action.Binding.Status == source.BackendBindingUnsupportedSignature || action.InputType == "" || seen[action.InputType] {
			continue
		}
		seen[action.InputType] = true
		types = append(types, action.InputType)
	}
	sort.Strings(types)
	return types
}

func actionUsesBoundInputDecoder(action BackendActionAdapter) bool {
	if action.Binding.Status != source.BackendBindingBound {
		return false
	}
	return action.Binding.Signature == source.BackendSignatureActionForm || action.Binding.Signature == source.BackendSignatureActionFormPtr
}

func boundActionDecoderDecl(action BackendActionAdapter) *ast.FuncDecl {
	inputType := sel(action.BackendAlias, action.Binding.InputType)
	stmts := []ast.Stmt{
		define([]ast.Expr{id("input")}, &ast.CompositeLit{Type: inputType}),
	}
	for index, field := range action.Binding.InputFields {
		stmts = append(stmts, boundActionFieldDecodeStmts(index, field)...)
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("input"), id("nil")}})
	return funcDecl(boundActionDecoderName(action), []*ast.Field{
		{Names: []*ast.Ident{id("values")}, Type: sel("gowdkform", "Values")},
	}, []*ast.Field{{Type: inputType}, {Type: id("error")}}, stmts)
}

func boundActionFieldDecodeStmts(index int, field source.BackendInputField) []ast.Stmt {
	value := id(fmtFieldValueName(index))
	switch field.Type {
	case "string":
		return boundActionScalarFieldDecodeStmts(value, field, call(sel("gowdkform", "String"), id("values"), stringLit(field.FormName)), value)
	case "bool":
		return boundActionScalarFieldDecodeStmts(value, field, call(sel("gowdkform", "Bool"), id("values"), stringLit(field.FormName)), value)
	case "int", "int8", "int16", "int32", "int64":
		return boundActionScalarFieldDecodeStmts(value, field, call(sel("gowdkform", "Int"), id("values"), stringLit(field.FormName), intLit(inputIntegerBitSize(field.Type))), convertIfNeeded(field.Type, value))
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return boundActionScalarFieldDecodeStmts(value, field, call(sel("gowdkform", "Uint"), id("values"), stringLit(field.FormName), intLit(inputIntegerBitSize(field.Type))), convertIfNeeded(field.Type, value))
	case "[]string":
		return []ast.Stmt{
			define([]ast.Expr{value}, call(sel("gowdkform", "Strings"), id("values"), stringLit(field.FormName))),
			&ast.IfStmt{
				Cond: &ast.BinaryExpr{X: call(id("len"), value), Op: token.GTR, Y: intLit(0)},
				Body: block(assign([]ast.Expr{selExpr(id("input"), field.FieldName)}, value)),
			},
		}
	default:
		return nil
	}
}

func boundActionScalarFieldDecodeStmts(value *ast.Ident, field source.BackendInputField, decode ast.Expr, assignment ast.Expr) []ast.Stmt {
	return []ast.Stmt{
		define([]ast.Expr{value, id("ok"), id("err")}, decode),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("input"), id("err")}}),
		},
		&ast.IfStmt{
			Cond: id("ok"),
			Body: block(assign([]ast.Expr{selExpr(id("input"), field.FieldName)}, assignment)),
		},
	}
}

func fmtFieldValueName(index int) string {
	return "field" + strconv.Itoa(index)
}

func inputIntegerBitSize(value string) int {
	switch value {
	case "int8", "uint8":
		return 8
	case "int16", "uint16":
		return 16
	case "int32", "uint32":
		return 32
	case "int64", "uint64":
		return 64
	default:
		return 0
	}
}

func convertIfNeeded(goType string, value ast.Expr) ast.Expr {
	if goType == "string" || goType == "bool" || goType == "[]string" {
		return value
	}
	return call(id(goType), value)
}

func actionDecoderName(action BackendActionAdapter) string {
	return "decode" + source.ExportedIdentifier(action.PageID, "Action") + source.ExportedIdentifier(action.ActionName, "Action") + "Input"
}

func boundActionDecoderName(action BackendActionAdapter) string {
	return "decode" + source.ExportedIdentifier(action.PageID, "Action") + source.ExportedIdentifier(action.ActionName, "Action") + "BoundInput"
}

func backendNotImplementedStmts(binding source.BackendBinding, kind string) []ast.Stmt {
	message := strings.TrimSpace(binding.Message)
	if message == "" {
		message = "GOWDK " + kind + " handler is not implemented"
	}
	return []ast.Stmt{writeNoStoreErrorStmt(sel("http", "StatusNotImplemented"), message)}
}

func writeNoStoreErrorStmt(status ast.Expr, message string) ast.Stmt {
	return writeNoStoreErrorExprStmt(status, stringLit(message))
}

func writeNoStoreErrorExprStmt(status ast.Expr, message ast.Expr) ast.Stmt {
	return exprStmt(call(sel("gowdkresponse", "WriteNoStoreError"), id("response"), status, message))
}

func writeNoStoreHandlerErrorExprStmt(err ast.Expr, fallbackStatus ast.Expr) ast.Stmt {
	return exprStmt(call(sel("gowdkresponse", "WriteNoStoreHandlerError"), id("response"), err, fallbackStatus))
}

func writeNoStoreJSONErrorStmt(status ast.Expr, message string) ast.Stmt {
	return exprStmt(call(sel("gowdkresponse", "WriteNoStoreJSONError"), id("response"), status, stringLit(message)))
}

func writeNoStoreHandlerJSONErrorExprStmt(err ast.Expr, fallbackStatus ast.Expr) ast.Stmt {
	return exprStmt(call(sel("gowdkresponse", "WriteNoStoreHandlerJSONError"), id("response"), err, fallbackStatus))
}

func handlerErrorMessageExpr(err ast.Expr, fallbackStatus ast.Expr) ast.Expr {
	return call(sel("gowdkresponse", "HandlerErrorMessage"), err, fallbackStatus)
}

func writeNoStoreHTTPStmt(result ast.Expr) ast.Stmt {
	return assign([]ast.Expr{id("_")}, call(sel("gowdkresponse", "WriteNoStoreHTTP"), id("response"), result))
}

func setNoStoreHeaderStmt() ast.Stmt {
	return exprStmt(call(selExpr(call(selExpr(id("response"), "Header")), "Set"), stringLit("Cache-Control"), stringLit("no-store")))
}

func ifErrReturnInvalidForm() ast.Stmt {
	return &ast.IfStmt{
		Cond: notNil("err"),
		Body: block(
			writeNoStoreErrorStmt(sel("http", "StatusBadRequest"), "invalid form"),
			returnBool(true),
		),
	}
}

func ifErrReturnInvalidJSONForm() ast.Stmt {
	return &ast.IfStmt{
		Cond: notNil("err"),
		Body: block(
			writeNoStoreJSONErrorStmt(sel("http", "StatusBadRequest"), "invalid form"),
			returnBool(true),
		),
	}
}

func trimHeaderCall(name string) ast.Expr {
	return call(sel("strings", "TrimSpace"), call(selExpr(selExpr(id("request"), "Header"), "Get"), stringLit(name)))
}

func formSchemaExpr(fields []string) ast.Expr {
	elts := make([]ast.Expr, 0, len(fields))
	for _, field := range fields {
		elts = append(elts, &ast.CompositeLit{
			Elts: []ast.Expr{keyValue("Name", stringLit(field))},
		})
	}
	return &ast.CompositeLit{
		Type: sel("gowdkform", "Schema"),
		Elts: []ast.Expr{keyValue("Fields", &ast.CompositeLit{
			Type: &ast.ArrayType{Elt: sel("gowdkform", "Field")},
			Elts: elts,
		})},
	}
}

func stringSliceExpr(values []string) ast.Expr {
	if len(values) == 0 {
		return id("nil")
	}
	elts := make([]ast.Expr, 0, len(values))
	for _, value := range values {
		elts = append(elts, stringLit(value))
	}
	return &ast.CompositeLit{
		Type: &ast.ArrayType{Elt: id("string")},
		Elts: elts,
	}
}

func actionParams() []*ast.Field {
	return []*ast.Field{
		{Names: []*ast.Ident{id("response")}, Type: sel("http", "ResponseWriter")},
		{Names: []*ast.Ident{id("request")}, Type: &ast.StarExpr{X: sel("http", "Request")}},
	}
}

func boolResults() []*ast.Field {
	return []*ast.Field{{Type: id("bool")}}
}

func namedBoolResults(name string) []*ast.Field {
	return []*ast.Field{{Names: []*ast.Ident{id(name)}, Type: id("bool")}}
}

func funcDecl(name string, params []*ast.Field, results []*ast.Field, stmts []ast.Stmt) *ast.FuncDecl {
	return &ast.FuncDecl{
		Name: id(name),
		Type: &ast.FuncType{
			Params:  &ast.FieldList{List: params},
			Results: &ast.FieldList{List: results},
		},
		Body: &ast.BlockStmt{List: stmts},
	}
}

func define(left []ast.Expr, right ...ast.Expr) ast.Stmt {
	return &ast.AssignStmt{Lhs: left, Tok: token.DEFINE, Rhs: right}
}

func assign(left []ast.Expr, right ...ast.Expr) ast.Stmt {
	return &ast.AssignStmt{Lhs: left, Tok: token.ASSIGN, Rhs: right}
}

func returnBool(value bool) ast.Stmt {
	return &ast.ReturnStmt{Results: []ast.Expr{id(strconv.FormatBool(value))}}
}

func notNil(name string) ast.Expr {
	return &ast.BinaryExpr{X: id(name), Op: token.NEQ, Y: id("nil")}
}

func block(stmts ...ast.Stmt) *ast.BlockStmt {
	return &ast.BlockStmt{List: stmts}
}

func exprStmt(expr ast.Expr) ast.Stmt {
	return &ast.ExprStmt{X: expr}
}

func call(fun ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{Fun: fun, Args: args}
}

func sel(parts ...string) ast.Expr {
	if len(parts) == 0 {
		return id("")
	}
	var expr ast.Expr = id(parts[0])
	for _, part := range parts[1:] {
		expr = selExpr(expr, part)
	}
	return expr
}

func selExpr(expr ast.Expr, name string) *ast.SelectorExpr {
	return &ast.SelectorExpr{X: expr, Sel: id(name)}
}

func keyValue(key string, value ast.Expr) ast.Expr {
	return &ast.KeyValueExpr{Key: id(key), Value: value}
}

func id(name string) *ast.Ident {
	return ast.NewIdent(name)
}

func stringLit(value string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(value)}
}

func intLit(value int) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(value)}
}

func int64Lit(value int64) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(value, 10)}
}

func goString(value string) string {
	return strconv.Quote(value)
}

func quote(value string) string {
	return strconv.Quote(path.Clean("/" + value))
}

func cleanRoutePath(value string) string {
	return path.Clean("/" + value)
}
