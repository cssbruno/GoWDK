package appgen

func backendAppPackageSource(options Options) (string, error) {
	imports := backendRuntimeImportMap(options)
	imports["http"] = "net/http"
	return printGoFile("gowdkapp", imports, append(backendShellDecls(options), backendGeneratedDecls(options)...))
}

func backendRuntimeImportSource(options Options) string {
	return importSpecSource(backendRuntimeImportMap(options))
}

func backendRuntimeImportMap(options Options) map[string]string {
	imports := map[string]string{}
	adapter := backendAdapterIR(options)
	contractExposures := adapter.ContractExposures
	routableContracts := routableContractExposures(contractExposures)
	executableContracts := executableContractExposures(contractExposures)
	if hasBackendRoutes(options) {
		imports["gowdkruntime"] = "github.com/cssbruno/gowdk/runtime/app"
	}
	if securityHeadersExpr(options) != nil {
		imports["strings"] = "strings"
	}
	if len(adapter.Registrations) > 0 || len(routableContracts) > 0 {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
	}
	if len(executableContracts) > 0 {
		imports["context"] = "context"
		imports["gowdkcontracts"] = "github.com/cssbruno/gowdk/runtime/contracts"
	}
	if len(executableCommandContractExposures(contractExposures)) > 0 {
		imports["sync"] = "sync"
	}
	if contractExposuresUseForm(executableContracts) {
		imports["gowdkform"] = "github.com/cssbruno/gowdk/runtime/form"
	}
	if contractExposuresParseForm(executableContracts) {
		imports["strings"] = "strings"
	}
	if len(adapter.Actions) > 0 {
		imports["path"] = "path"
	}
	if actionsParseForm(adapter.Actions) {
		imports["strings"] = "strings"
	}
	if actionsUseForm(adapter.Actions) {
		imports["gowdkform"] = "github.com/cssbruno/gowdk/runtime/form"
	}
	if len(adapter.APIs) > 0 {
		imports["path"] = "path"
	}
	if fragmentsUseExactRoutes(adapter.Fragments) {
		imports["path"] = "path"
	}
	if fragmentsUseDynamicRoutes(adapter.Fragments) {
		imports["gowdkroute"] = "github.com/cssbruno/gowdk/runtime/route"
	}
	if actionsUseFragments(adapter.Actions) || fragmentsUseStaticFallback(adapter.Fragments) {
		imports["gowdkpartial"] = "github.com/cssbruno/gowdk/addons/partial"
	}
	if actionsUseValidation(adapter.Actions) {
		imports["gowdkvalidation"] = "github.com/cssbruno/gowdk/runtime/validation"
	}
	if generatedUsesGuards(options) {
		imports["gowdkauth"] = "github.com/cssbruno/gowdk/runtime/auth"
		imports["gowdkguard"] = "github.com/cssbruno/gowdk/runtime/guard"
	}
	if csrfEnabled(options) {
		imports["errors"] = "errors"
		imports["gowdkactions"] = "github.com/cssbruno/gowdk/addons/actions"
		imports["os"] = "os"
		imports["strings"] = "strings"
	}
	for importPath, alias := range backendImports(adapter, nil) {
		imports[alias] = importPath
	}
	for importPath, alias := range backendContractImports(executableContracts) {
		imports[alias] = importPath
	}

	return imports
}
