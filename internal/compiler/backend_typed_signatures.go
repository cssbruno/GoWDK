package compiler

import (
	"go/types"

	"github.com/cssbruno/gowdk/internal/source"
)

func backendTypedSignature(function *types.Signature, pkg *types.Package) (source.BackendSignatureKind, string, bool, []source.BackendInputField, string, bool, []source.BackendResultField, string) {
	if kind, inputType, inputPointer, inputFields, supportMessage, ok := typedActionSignature(function, pkg); ok {
		return kind, inputType, inputPointer, inputFields, "", false, nil, supportMessage
	}
	if isTypedAPISignature(function) {
		return source.BackendSignatureAPI, "", false, nil, "", false, nil, ""
	}
	if kind, inputType, inputPointer, inputFields, resultType, resultPointer, resultFields, supportMessage, ok := typedAPISignature(function, pkg); ok {
		return kind, inputType, inputPointer, inputFields, resultType, resultPointer, resultFields, supportMessage
	}
	if signature, resultType, resultPointer, resultFields, supportMessage, ok := typedLoadSignature(function, pkg); ok {
		return signature, "", false, nil, resultType, resultPointer, resultFields, supportMessage
	}
	return "", "", false, nil, "", false, nil, ""
}

func typedActionSignature(function *types.Signature, pkg *types.Package) (source.BackendSignatureKind, string, bool, []source.BackendInputField, string, bool) {
	if function == nil || function.Params() == nil || function.Results() == nil {
		return "", "", false, nil, "", false
	}
	params := function.Params()
	results := function.Results()
	if results.Len() != 2 {
		return "", "", false, nil, "", false
	}
	if !isTypedNamed(results.At(0).Type(), responseImportPath, "Response") || !isTypedError(results.At(1).Type()) {
		return "", "", false, nil, "", false
	}
	if params.Len() != 1 && params.Len() != 2 {
		return "", "", false, nil, "", false
	}
	if !isTypedNamed(params.At(0).Type(), contextImportPath, "Context") {
		return "", "", false, nil, "", false
	}
	if params.Len() == 1 {
		return source.BackendSignatureAction0, "", false, nil, "", true
	}
	second := params.At(1).Type()
	if isTypedNamed(second, formImportPath, "Values") {
		return source.BackendSignatureActionValues, "", false, nil, "", true
	}
	if isTypedNamed(second, formImportPath, "Data") {
		return source.BackendSignatureActionData, "", false, nil, "", true
	}
	inputName, inputPointer, inputType, ok := typedLocalInputType(second, pkg)
	if !ok {
		return "", "", false, nil, "", false
	}
	inputStruct := backendTypedInputStruct(inputName, inputType)
	if inputStruct.Message != "" {
		return "", inputName, inputPointer, nil, inputStruct.Message, true
	}
	kind := source.BackendSignatureActionForm
	if inputPointer {
		kind = source.BackendSignatureActionFormPtr
	}
	return kind, inputName, inputPointer, append([]source.BackendInputField(nil), inputStruct.Fields...), "", true
}

func isTypedAPISignature(function *types.Signature) bool {
	if function == nil || function.Params() == nil || function.Results() == nil {
		return false
	}
	params := function.Params()
	results := function.Results()
	if params.Len() != 2 || results.Len() != 2 {
		return false
	}
	return isTypedNamed(params.At(0).Type(), contextImportPath, "Context") &&
		isTypedPointerToNamed(params.At(1).Type(), httpImportPath, "Request") &&
		isTypedNamed(results.At(0).Type(), responseImportPath, "Response") &&
		isTypedError(results.At(1).Type())
}

func typedAPISignature(function *types.Signature, pkg *types.Package) (source.BackendSignatureKind, string, bool, []source.BackendInputField, string, bool, []source.BackendResultField, string, bool) {
	if function == nil || function.Params() == nil || function.Results() == nil {
		return "", "", false, nil, "", false, nil, "", false
	}
	params := function.Params()
	results := function.Results()
	if params.Len() != 1 && params.Len() != 2 {
		return "", "", false, nil, "", false, nil, "", false
	}
	if !isTypedNamed(params.At(0).Type(), contextImportPath, "Context") {
		return "", "", false, nil, "", false, nil, "", false
	}
	if results.Len() != 2 || !isTypedError(results.At(1).Type()) {
		return "", "", false, nil, "", false, nil, "", false
	}
	resultType, resultPointer, resultNamed, ok := typedLocalResultType(results.At(0).Type(), pkg)
	if !ok {
		return "", "", false, nil, "", false, nil, "", false
	}
	resultStruct := backendTypedAPIResultStruct(resultType, resultNamed)
	if resultStruct.Message != "" {
		return "", "", false, nil, resultType, resultPointer, nil, resultStruct.Message, true
	}
	if params.Len() == 1 {
		return source.BackendSignatureAPI0, "", false, nil, resultType, resultPointer, append([]source.BackendResultField(nil), resultStruct.Fields...), "", true
	}
	inputName, inputPointer, inputType, ok := typedLocalInputType(params.At(1).Type(), pkg)
	if !ok {
		return "", "", false, nil, "", false, nil, "", false
	}
	inputStruct := backendTypedAPIInputStruct(inputName, inputType)
	if inputStruct.Message != "" {
		return "", inputName, inputPointer, nil, resultType, resultPointer, nil, inputStruct.Message, true
	}
	kind := source.BackendSignatureAPIInput
	if inputPointer {
		kind = source.BackendSignatureAPIInputPtr
	}
	return kind, inputName, inputPointer, append([]source.BackendInputField(nil), inputStruct.Fields...), resultType, resultPointer, append([]source.BackendResultField(nil), resultStruct.Fields...), "", true
}

func typedLoadSignature(function *types.Signature, pkg *types.Package) (source.BackendSignatureKind, string, bool, []source.BackendResultField, string, bool) {
	if function == nil || function.Params() == nil || function.Results() == nil {
		return "", "", false, nil, "", false
	}
	params := function.Params()
	results := function.Results()
	if params.Len() != 1 || !isTypedLoadContext(params.At(0).Type()) {
		return "", "", false, nil, "", false
	}
	if results.Len() == 1 && isTypedMapStringAny(results.At(0).Type()) {
		return source.BackendSignatureLoad, "", false, nil, "", true
	}
	if results.Len() == 2 && isTypedMapStringAny(results.At(0).Type()) && isTypedError(results.At(1).Type()) {
		return source.BackendSignatureLoadError, "", false, nil, "", true
	}
	resultCount := results.Len()
	if resultCount != 1 && resultCount != 2 {
		return "", "", false, nil, "", false
	}
	if resultCount == 2 && !isTypedError(results.At(1).Type()) {
		return "", "", false, nil, "", false
	}
	resultType, resultPointer, namedType, ok := typedLocalResultType(results.At(0).Type(), pkg)
	if !ok {
		return "", "", false, nil, "", false
	}
	resultStruct := backendTypedResultStruct(resultType, namedType)
	signature := source.BackendSignatureLoadStruct
	if resultCount == 2 {
		signature = source.BackendSignatureLoadStructError
	}
	if resultStruct.Message != "" {
		return "", resultType, resultPointer, nil, resultStruct.Message, true
	}
	return signature, resultType, resultPointer, append([]source.BackendResultField(nil), resultStruct.Fields...), "", true
}

func typedLocalInputType(typ types.Type, pkg *types.Package) (string, bool, types.Type, bool) {
	inputPointer := false
	typ = types.Unalias(typ)
	if pointer, ok := typ.(*types.Pointer); ok {
		inputPointer = true
		typ = types.Unalias(pointer.Elem())
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj() == nil || !named.Obj().Exported() {
		return "", false, nil, false
	}
	if pkg == nil || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != pkg.Path() {
		return "", false, nil, false
	}
	if _, ok := named.Underlying().(*types.Struct); !ok {
		return "", false, nil, true
	}
	return named.Obj().Name(), inputPointer, named, true
}

func typedLocalResultType(typ types.Type, pkg *types.Package) (string, bool, types.Type, bool) {
	resultPointer := false
	typ = types.Unalias(typ)
	if pointer, ok := typ.(*types.Pointer); ok {
		resultPointer = true
		typ = types.Unalias(pointer.Elem())
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj() == nil || !named.Obj().Exported() {
		return "", false, nil, false
	}
	if pkg == nil || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != pkg.Path() {
		return "", false, nil, false
	}
	return named.Obj().Name(), resultPointer, named, true
}

func isTypedNamed(typ types.Type, importPath, name string) bool {
	named, ok := types.Unalias(typ).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Name() != name {
		return false
	}
	pkg := named.Obj().Pkg()
	return pkg != nil && pkg.Path() == importPath
}

func isTypedPointerToNamed(typ types.Type, importPath, name string) bool {
	pointer, ok := types.Unalias(typ).(*types.Pointer)
	return ok && isTypedNamed(pointer.Elem(), importPath, name)
}

func isTypedLoadContext(typ types.Type) bool {
	return isTypedNamed(typ, ssrImportPath, "LoadContext") ||
		isTypedNamed(typ, guardImportPath, "Context")
}

func isTypedError(typ types.Type) bool {
	errorObj := types.Universe.Lookup("error")
	return errorObj != nil && types.Identical(types.Unalias(typ), errorObj.Type())
}

func isTypedMapStringAny(typ types.Type) bool {
	mapType, ok := types.Unalias(typ).Underlying().(*types.Map)
	if !ok {
		return false
	}
	key, ok := types.Unalias(mapType.Key()).Underlying().(*types.Basic)
	if !ok || key.Kind() != types.String {
		return false
	}
	value, ok := types.Unalias(mapType.Elem()).Underlying().(*types.Interface)
	return ok && value.Empty()
}
