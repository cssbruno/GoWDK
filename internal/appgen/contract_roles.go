package appgen

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/contractscan"
	"github.com/cssbruno/gowdk/internal/source"
	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

const (
	workerDirName      = "cmd/worker"
	cronDirName        = "cmd/cron"
	workerMainFileName = workerDirName + "/main.go"
	cronMainFileName   = cronDirName + "/main.go"
	roleFileName       = appPackageDirName + "/contracts.go"
	cronFileName       = appPackageDirName + "/cron.go"
)

type contractRoleRegistration struct {
	ImportPath string
	Alias      string
	Function   string
}

type contractCronJobPlan struct {
	Name       string
	Schedule   string
	Overlap    string
	Missed     string
	TypeAlias  string
	TypeName   string
	RunFunc    string
	Contract   string
	ImportPath string
}

// GenerateContractWorker writes a standalone generated worker role app.
func GenerateContractWorker(appDir string, report contractscan.Report, config gowdk.ContractWorkerConfig) (result Result, err error) {
	defer recoverGeneratedIdentifierError(&err)
	if strings.TrimSpace(appDir) == "" {
		return Result{}, fmt.Errorf("generated worker app directory is required")
	}
	if !serviceRefConfigured(config.EventSource) {
		return Result{}, fmt.Errorf("contract worker target requires Worker.EventSource")
	}
	if err := validateRoleProvider("Worker.EventSource", config.EventSource); err != nil {
		return Result{}, err
	}
	if err := validateOptionalRoleProvider("Worker.SeenStore", config.SeenStore); err != nil {
		return Result{}, err
	}
	if err := validateOptionalRoleProvider("Worker.Backoff", config.Backoff); err != nil {
		return Result{}, err
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return Result{}, err
	}
	module, err := currentAppModule()
	if err != nil {
		return Result{}, fmt.Errorf("cannot determine the app Go module for contract worker imports: %w", err)
	}
	registrations, contracts, importPaths, err := workerRoleRegistrations(module, report)
	if err != nil {
		return Result{}, err
	}
	if len(registrations) == 0 {
		return Result{}, fmt.Errorf("contract worker target has no worker-role event registrations")
	}
	providerAliases := roleProviderAliases(config.EventSource, config.SeenStore, config.Backoff)
	for importPath := range providerAliases {
		importPaths[importPath] = true
	}
	if err := os.MkdirAll(absApp, 0o755); err != nil {
		return Result{}, err
	}
	modulePayload, err := moduleSourceForImportPaths(importPaths)
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, modFileName), []byte(modulePayload)); err != nil {
		return Result{}, err
	}
	roleSource, err := rolePackageSource(registrations)
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, roleFileName), []byte(roleSource)); err != nil {
		return Result{}, err
	}
	mainSource, err := workerMainSource(config, providerAliases)
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, workerMainFileName), []byte(mainSource)); err != nil {
		return Result{}, err
	}
	return Result{
		AppDir:      absApp,
		MainPath:    filepath.Join(absApp, workerMainFileName),
		PackagePath: filepath.Join(absApp, roleFileName),
		ModulePath:  filepath.Join(absApp, modFileName),
		Role:        string(runtimecontracts.RoleWorker),
		Contracts:   contracts,
	}, nil
}

// GenerateContractCron writes a standalone generated cron role app.
func GenerateContractCron(appDir string, report contractscan.Report, config gowdk.ContractCronConfig) (result Result, err error) {
	defer recoverGeneratedIdentifierError(&err)
	if strings.TrimSpace(appDir) == "" {
		return Result{}, fmt.Errorf("generated cron app directory is required")
	}
	if len(config.Jobs) == 0 {
		return Result{}, fmt.Errorf("contract cron target requires Cron.Jobs")
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return Result{}, err
	}
	module, err := currentAppModule()
	if err != nil {
		return Result{}, fmt.Errorf("cannot determine the app Go module for contract cron imports: %w", err)
	}
	registrations, jobs, importPaths, err := cronRolePlan(module, report, config)
	if err != nil {
		return Result{}, err
	}
	if err := validateCronJobSchedules(jobs); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(absApp, 0o755); err != nil {
		return Result{}, err
	}
	modulePayload, err := moduleSourceForImportPaths(importPaths)
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, modFileName), []byte(modulePayload)); err != nil {
		return Result{}, err
	}
	roleSource, err := rolePackageSource(registrations)
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, roleFileName), []byte(roleSource)); err != nil {
		return Result{}, err
	}
	cronSource, err := cronPackageSource(jobs)
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, cronFileName), []byte(cronSource)); err != nil {
		return Result{}, err
	}
	mainSource, err := cronMainSource()
	if err != nil {
		return Result{}, err
	}
	if err := writeFileIfChanged(filepath.Join(absApp, cronMainFileName), []byte(mainSource)); err != nil {
		return Result{}, err
	}
	jobNames := make([]string, 0, len(jobs))
	for _, job := range jobs {
		jobNames = append(jobNames, job.Contract)
	}
	return Result{
		AppDir:      absApp,
		MainPath:    filepath.Join(absApp, cronMainFileName),
		PackagePath: filepath.Join(absApp, roleFileName),
		ModulePath:  filepath.Join(absApp, modFileName),
		Role:        string(runtimecontracts.RoleCron),
		Jobs:        jobNames,
	}, nil
}

func validateCronJobSchedules(jobs []contractCronJobPlan) error {
	scheduled := make([]runtimecontracts.ScheduledJob, 0, len(jobs))
	for _, job := range jobs {
		scheduled = append(scheduled, runtimecontracts.ScheduledJob{
			Name:            job.Contract,
			Schedule:        job.Schedule,
			OverlapPolicy:   job.Overlap,
			MissedRunPolicy: job.Missed,
			Run:             func(context.Context) error { return nil },
		})
	}
	return runtimecontracts.ValidateScheduledJobs(scheduled)
}

func workerRoleRegistrations(module appModuleInfo, report contractscan.Report) ([]contractRoleRegistration, []string, map[string]bool, error) {
	seen := map[string]bool{}
	contracts := map[string]bool{}
	importPaths := map[string]bool{}
	var registrations []contractRoleRegistration
	for _, contract := range report.Contracts {
		if contract.Kind != runtimecontracts.Event || !contractRolesAllow(contract.Roles, string(runtimecontracts.RoleWorker)) {
			continue
		}
		registration, err := contractRegistration(module, report, contract)
		if err != nil {
			return nil, nil, nil, err
		}
		key := registration.ImportPath + "\x00" + registration.Function
		if !seen[key] {
			seen[key] = true
			importPaths[registration.ImportPath] = true
			registrations = append(registrations, registration)
		}
		contracts[contractDisplayName(module, report, contract)] = true
	}
	sortRoleRegistrations(registrations)
	names := sortedStringSet(contracts)
	return registrations, names, importPaths, nil
}

func cronRolePlan(module appModuleInfo, report contractscan.Report, config gowdk.ContractCronConfig) ([]contractRoleRegistration, []contractCronJobPlan, map[string]bool, error) {
	seenRegistrations := map[string]bool{}
	importPaths := map[string]bool{}
	var registrations []contractRoleRegistration
	usedFuncs := map[string]bool{}
	var jobs []contractCronJobPlan
	for _, configured := range config.Jobs {
		contract, err := matchCronJob(module, report, configured.Type)
		if err != nil {
			return nil, nil, nil, err
		}
		registration, err := contractRegistration(module, report, contract)
		if err != nil {
			return nil, nil, nil, err
		}
		key := registration.ImportPath + "\x00" + registration.Function
		if !seenRegistrations[key] {
			seenRegistrations[key] = true
			importPaths[registration.ImportPath] = true
			registrations = append(registrations, registration)
		}
		typeImportPath := strings.TrimSpace(contract.TypeImportPath)
		if typeImportPath == "" {
			typeImportPath = registration.ImportPath
		}
		importPaths[typeImportPath] = true
		typeName := contractTypeIdent(contract.Type)
		contractName := contractDisplayName(module, report, contract)
		runFunc := uniqueCronRunFunc(contractName, usedFuncs)
		jobs = append(jobs, contractCronJobPlan{
			Name:       contractName,
			Schedule:   configured.Schedule,
			Overlap:    configured.OverlapPolicy,
			Missed:     configured.MissedRunPolicy,
			TypeAlias:  "",
			TypeName:   typeName,
			RunFunc:    runFunc,
			Contract:   contractName,
			ImportPath: typeImportPath,
		})
	}
	sortRoleRegistrations(registrations)
	aliases := roleImportAliases(importPaths, nil)
	for index := range registrations {
		registrations[index].Alias = aliases[registrations[index].ImportPath]
	}
	for index := range jobs {
		jobs[index].TypeAlias = aliases[jobs[index].ImportPath]
	}
	return registrations, jobs, importPaths, nil
}

func contractRegistration(module appModuleInfo, report contractscan.Report, contract contractscan.Contract) (contractRoleRegistration, error) {
	if strings.TrimSpace(contract.Register) == "" {
		return contractRoleRegistration{}, fmt.Errorf("contract %q in %s is missing a registry function", contract.Type, contract.Source)
	}
	importPath, err := contractRegisterImportPath(module, report, contract)
	if err != nil {
		return contractRoleRegistration{}, err
	}
	return contractRoleRegistration{ImportPath: importPath, Function: contract.Register}, nil
}

func contractRegisterImportPath(module appModuleInfo, report contractscan.Report, contract contractscan.Contract) (string, error) {
	if strings.TrimSpace(module.Path) == "" || strings.TrimSpace(module.Dir) == "" {
		return "", fmt.Errorf("cannot derive import path for contract %q: missing app module metadata", contract.Type)
	}
	sourcePath := filepath.Join(report.Root, filepath.FromSlash(contract.Source))
	sourceDir := filepath.Dir(sourcePath)
	rel, err := filepath.Rel(module.Dir, sourceDir)
	if err != nil {
		return "", fmt.Errorf("derive import path for contract %q: %w", contract.Type, err)
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return module.Path, nil
	}
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return "", fmt.Errorf("contract %q source %s is outside app module %s", contract.Type, contract.Source, module.Dir)
	}
	return strings.TrimRight(module.Path, "/") + "/" + rel, nil
}

func matchCronJob(module appModuleInfo, report contractscan.Report, configuredType string) (contractscan.Contract, error) {
	configuredType = strings.TrimSpace(configuredType)
	if configuredType == "" {
		return contractscan.Contract{}, fmt.Errorf("contract cron job is missing Type")
	}
	var matches []contractscan.Contract
	for _, contract := range report.Contracts {
		if contract.Kind != runtimecontracts.Job || !contractRolesAllow(contract.Roles, string(runtimecontracts.RoleCron)) {
			continue
		}
		if contractMatchesConfiguredType(module, report, contract, configuredType) {
			matches = append(matches, contract)
		}
	}
	switch len(matches) {
	case 0:
		return contractscan.Contract{}, fmt.Errorf("contract cron job %q was not found for role %q", configuredType, runtimecontracts.RoleCron)
	case 1:
		return matches[0], nil
	default:
		return contractscan.Contract{}, fmt.Errorf("contract cron job %q matched multiple scanned jobs; use a full import-path-qualified type", configuredType)
	}
}

func contractMatchesConfiguredType(module appModuleInfo, report contractscan.Report, contract contractscan.Contract, configuredType string) bool {
	labels := map[string]bool{
		contract.Type: true,
	}
	if contract.Package != "" {
		labels[contract.Package+"."+contract.Type] = true
	}
	if importPath, err := contractRegisterImportPath(module, report, contract); err == nil {
		labels[importPath+"."+contractTypeIdent(contract.Type)] = true
	}
	if contract.TypeImportPath != "" {
		labels[contract.TypeImportPath+"."+contractTypeIdent(contract.Type)] = true
	}
	return labels[configuredType]
}

func contractDisplayName(module appModuleInfo, report contractscan.Report, contract contractscan.Contract) string {
	typeName := contractTypeIdent(contract.Type)
	if contract.TypeImportPath != "" {
		return contract.TypeImportPath + "." + typeName
	}
	if importPath, err := contractRegisterImportPath(module, report, contract); err == nil {
		return importPath + "." + typeName
	}
	if contract.Package != "" {
		return contract.Package + "." + typeName
	}
	return typeName
}

func contractTypeIdent(name string) string {
	name = strings.TrimSpace(name)
	if index := strings.LastIndex(name, "."); index >= 0 {
		return name[index+1:]
	}
	return name
}

func contractRolesAllow(roles []string, role string) bool {
	if len(roles) == 0 {
		return true
	}
	for _, candidate := range roles {
		if candidate == role || candidate == string(runtimecontracts.RoleAny) {
			return true
		}
	}
	return false
}

func sortRoleRegistrations(registrations []contractRoleRegistration) {
	sort.Slice(registrations, func(i, j int) bool {
		if registrations[i].ImportPath != registrations[j].ImportPath {
			return registrations[i].ImportPath < registrations[j].ImportPath
		}
		return registrations[i].Function < registrations[j].Function
	})
}

func sortedStringSet(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func rolePackageSource(registrations []contractRoleRegistration) (string, error) {
	importPaths := map[string]bool{}
	for _, registration := range registrations {
		importPaths[registration.ImportPath] = true
	}
	aliases := roleImportAliases(importPaths, nil)
	for index := range registrations {
		registrations[index].Alias = aliases[registrations[index].ImportPath]
	}
	imports := map[string]string{
		"context":        "context",
		"gowdkcontracts": "github.com/cssbruno/gowdk/runtime/contracts",
		"sync":           "sync",
	}
	for _, registration := range registrations {
		imports[registration.Alias] = registration.ImportPath
	}
	return printGoFile("gowdkapp", imports, []ast.Decl{
		roleRegistryStateDecl(),
		roleNewContractRegistryDecl(registrations),
		roleContractRegistryDecl(),
		roleRunContractEventWorkerDecl(),
		roleRunContractEventWorkerWithOptionsDecl(),
		roleRunContractEventWorkerWithSeenStoreDecl(),
		roleRunContractEventWorkerWithSeenStoreAndOptionsDecl(),
	})
}

func roleImportAliases(importPaths map[string]bool, preferred map[string]string) map[string]string {
	paths := make([]string, 0, len(importPaths))
	for importPath := range importPaths {
		if strings.TrimSpace(importPath) != "" {
			paths = append(paths, importPath)
		}
	}
	sort.Strings(paths)
	used := generatedImportAliasUseCounts()
	for _, alias := range []string{"gowdkapp", "log", "signal", "syscall"} {
		used[alias] = 1
	}
	aliases := map[string]string{}
	for _, importPath := range paths {
		base := ""
		if preferred != nil {
			base = preferred[importPath]
		}
		if base == "" {
			base = path.Base(importPath)
		}
		base = safeImportAlias(base)
		if base == "" {
			base = "contractpkg"
		}
		aliases[importPath] = nextImportAlias(base, used)
	}
	return aliases
}

func roleRegistryStateDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{
		&ast.ValueSpec{Names: []*ast.Ident{id("contractRegistryOnce")}, Type: sel("sync", "Once")},
		&ast.ValueSpec{Names: []*ast.Ident{id("contractRegistry")}, Type: &ast.StarExpr{X: sel("gowdkcontracts", "Registry")}},
	}}
}

func roleNewContractRegistryDecl(registrations []contractRoleRegistration) ast.Decl {
	stmts := []ast.Stmt{define([]ast.Expr{id("contractRegistry")}, call(sel("gowdkcontracts", "NewRegistry")))}
	for _, registration := range registrations {
		stmts = append(stmts, exprStmt(call(sel(registration.Alias, registration.Function), id("contractRegistry"))))
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("contractRegistry")}})
	return funcDecl("NewContractRegistry", nil, []*ast.Field{{Type: &ast.StarExpr{X: sel("gowdkcontracts", "Registry")}}}, stmts)
}

func roleContractRegistryDecl() ast.Decl {
	return funcDecl("ContractRegistry", nil, []*ast.Field{{Type: &ast.StarExpr{X: sel("gowdkcontracts", "Registry")}}}, []ast.Stmt{
		exprStmt(call(selExpr(id("contractRegistryOnce"), "Do"), &ast.FuncLit{
			Type: &ast.FuncType{Params: &ast.FieldList{}},
			Body: block(assign([]ast.Expr{id("contractRegistry")}, call(id("NewContractRegistry")))),
		})),
		&ast.ReturnStmt{Results: []ast.Expr{id("contractRegistry")}},
	})
}

func roleRunContractEventWorkerDecl() ast.Decl {
	return funcDecl("RunContractEventWorker", []*ast.Field{
		{Names: []*ast.Ident{id("ctx")}, Type: sel("context", "Context")},
		{Names: []*ast.Ident{id("source")}, Type: sel("gowdkcontracts", "EventSource")},
	}, []*ast.Field{{Type: id("error")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{call(sel("gowdkcontracts", "RunEventWorker"), id("ctx"), call(id("NewContractRegistry")), id("source"))}},
	})
}

func roleRunContractEventWorkerWithOptionsDecl() ast.Decl {
	return funcDecl("RunContractEventWorkerWithOptions", []*ast.Field{
		{Names: []*ast.Ident{id("ctx")}, Type: sel("context", "Context")},
		{Names: []*ast.Ident{id("source")}, Type: sel("gowdkcontracts", "EventSource")},
		{Names: []*ast.Ident{id("options")}, Type: &ast.Ellipsis{Elt: sel("gowdkcontracts", "EventWorkerOption")}},
	}, []*ast.Field{{Type: id("error")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{&ast.CallExpr{
			Fun:      sel("gowdkcontracts", "RunEventWorkerWithOptions"),
			Args:     []ast.Expr{id("ctx"), call(id("NewContractRegistry")), id("source"), id("options")},
			Ellipsis: token.Pos(1),
		}}},
	})
}

func roleRunContractEventWorkerWithSeenStoreDecl() ast.Decl {
	return funcDecl("RunContractEventWorkerWithSeenStore", []*ast.Field{
		{Names: []*ast.Ident{id("ctx")}, Type: sel("context", "Context")},
		{Names: []*ast.Ident{id("source")}, Type: sel("gowdkcontracts", "EventSource")},
		{Names: []*ast.Ident{id("seen")}, Type: sel("gowdkcontracts", "SeenStore")},
	}, []*ast.Field{{Type: id("error")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{call(sel("gowdkcontracts", "RunEventWorkerWithSeenStore"), id("ctx"), call(id("NewContractRegistry")), id("source"), id("seen"))}},
	})
}

func roleRunContractEventWorkerWithSeenStoreAndOptionsDecl() ast.Decl {
	return funcDecl("RunContractEventWorkerWithSeenStoreAndOptions", []*ast.Field{
		{Names: []*ast.Ident{id("ctx")}, Type: sel("context", "Context")},
		{Names: []*ast.Ident{id("source")}, Type: sel("gowdkcontracts", "EventSource")},
		{Names: []*ast.Ident{id("seen")}, Type: sel("gowdkcontracts", "SeenStore")},
		{Names: []*ast.Ident{id("options")}, Type: &ast.Ellipsis{Elt: sel("gowdkcontracts", "EventWorkerOption")}},
	}, []*ast.Field{{Type: id("error")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{&ast.CallExpr{
			Fun:      sel("gowdkcontracts", "RunEventWorkerWithSeenStoreAndOptions"),
			Args:     []ast.Expr{id("ctx"), call(id("NewContractRegistry")), id("source"), id("seen"), id("options")},
			Ellipsis: token.Pos(1),
		}}},
	})
}

func workerMainSource(config gowdk.ContractWorkerConfig, providerAliases map[string]string) (string, error) {
	imports := map[string]string{
		"context":        "context",
		"gowdkapp":       "gowdk-generated-app/gowdkapp",
		"gowdkcontracts": "github.com/cssbruno/gowdk/runtime/contracts",
		"log":            "log",
		"os":             "os",
		"signal":         "os/signal",
		"syscall":        "syscall",
	}
	for importPath, alias := range providerAliases {
		imports[alias] = importPath
	}
	return printGoFile("main", imports, []ast.Decl{workerMainDecl(config, providerAliases)})
}

func workerMainDecl(config gowdk.ContractWorkerConfig, providerAliases map[string]string) ast.Decl {
	stmts := signalContextStmts()
	stmts = append(stmts, providerCallStmts("source", providerAliases[config.EventSource.ImportPath], config.EventSource.Function)...)
	stmts = append(stmts, define([]ast.Expr{id("workerOptions")}, &ast.CompositeLit{Type: &ast.ArrayType{Elt: sel("gowdkcontracts", "EventWorkerOption")}}))
	if serviceRefConfigured(config.Backoff) {
		stmts = append(stmts, providerCallStmts("backoff", providerAliases[config.Backoff.ImportPath], config.Backoff.Function)...)
		stmts = append(stmts, assign([]ast.Expr{id("workerOptions")}, call(id("append"), id("workerOptions"), call(sel("gowdkcontracts", "WithEventWorkerBackoff"), id("backoff")))))
	}
	if serviceRefConfigured(config.SeenStore) {
		stmts = append(stmts, providerCallStmts("seen", providerAliases[config.SeenStore.ImportPath], config.SeenStore.Function)...)
		stmts = append(stmts, workerRunStmt(true), returnIfRunErrStmt())
		return funcDecl("main", nil, nil, stmts)
	}
	stmts = append(stmts, workerRunStmt(false), returnIfRunErrStmt())
	return funcDecl("main", nil, nil, stmts)
}

func signalContextStmts() []ast.Stmt {
	return []ast.Stmt{
		define([]ast.Expr{id("ctx"), id("stop")}, call(sel("signal", "NotifyContext"), call(sel("context", "Background")), sel("os", "Interrupt"), sel("syscall", "SIGTERM"))),
		&ast.DeferStmt{Call: call(id("stop"))},
	}
}

func providerCallStmts(name string, alias string, function string) []ast.Stmt {
	return []ast.Stmt{
		define([]ast.Expr{id(name), id("err")}, call(sel(alias, function))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(exprStmt(call(sel("log", "Fatal"), id("err")))),
		},
	}
}

func workerRunStmt(seen bool) ast.Stmt {
	if seen {
		return assign([]ast.Expr{id("err")}, &ast.CallExpr{
			Fun:      sel("gowdkapp", "RunContractEventWorkerWithSeenStoreAndOptions"),
			Args:     []ast.Expr{id("ctx"), id("source"), id("seen"), id("workerOptions")},
			Ellipsis: token.Pos(1),
		})
	}
	return assign([]ast.Expr{id("err")}, &ast.CallExpr{
		Fun:      sel("gowdkapp", "RunContractEventWorkerWithOptions"),
		Args:     []ast.Expr{id("ctx"), id("source"), id("workerOptions")},
		Ellipsis: token.Pos(1),
	})
}

func returnIfRunErrStmt() ast.Stmt {
	return &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  notNilExpr(id("err")),
			Op: token.LAND,
			Y:  &ast.BinaryExpr{X: call(selExpr(id("ctx"), "Err")), Op: token.EQL, Y: id("nil")},
		},
		Body: block(exprStmt(call(sel("log", "Fatal"), id("err")))),
	}
}

func cronPackageSource(jobs []contractCronJobPlan) (string, error) {
	importPaths := map[string]bool{}
	for _, job := range jobs {
		importPaths[job.ImportPath] = true
	}
	aliases := roleImportAliases(importPaths, nil)
	for index := range jobs {
		jobs[index].TypeAlias = aliases[jobs[index].ImportPath]
	}
	imports := map[string]string{
		"context":        "context",
		"gowdkcontracts": "github.com/cssbruno/gowdk/runtime/contracts",
	}
	for _, job := range jobs {
		imports[job.TypeAlias] = job.ImportPath
	}
	decls := []ast.Decl{runContractCronDecl(jobs)}
	for _, job := range jobs {
		decls = append(decls, runContractCronJobDecl(job))
	}
	return printGoFile("gowdkapp", imports, decls)
}

func runContractCronDecl(jobs []contractCronJobPlan) ast.Decl {
	elts := make([]ast.Expr, 0, len(jobs))
	for _, job := range jobs {
		elts = append(elts, &ast.CompositeLit{
			Type: sel("gowdkcontracts", "ScheduledJob"),
			Elts: []ast.Expr{
				keyValue("Name", stringLit(job.Contract)),
				keyValue("Schedule", stringLit(job.Schedule)),
				keyValue("OverlapPolicy", stringLit(job.Overlap)),
				keyValue("MissedRunPolicy", stringLit(job.Missed)),
				keyValue("Run", id(job.RunFunc)),
			},
		})
	}
	return funcDecl("RunContractCron", []*ast.Field{{Names: []*ast.Ident{id("ctx")}, Type: sel("context", "Context")}}, []*ast.Field{{Type: id("error")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{call(sel("gowdkcontracts", "RunScheduledJobs"), id("ctx"), &ast.CompositeLit{
			Type: &ast.ArrayType{Elt: sel("gowdkcontracts", "ScheduledJob")},
			Elts: elts,
		})}},
	})
}

func runContractCronJobDecl(job contractCronJobPlan) ast.Decl {
	jobType := sel(job.TypeAlias, job.TypeName)
	return funcDecl(job.RunFunc, []*ast.Field{{Names: []*ast.Ident{id("ctx")}, Type: sel("context", "Context")}}, []*ast.Field{{Type: id("error")}}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{
			call(&ast.IndexExpr{X: sel("gowdkcontracts", "ExecuteJobForRole"), Index: jobType}, id("ctx"), call(id("NewContractRegistry")), sel("gowdkcontracts", "RoleCron"), &ast.CompositeLit{Type: jobType}),
		}},
	})
}

func cronMainSource() (string, error) {
	return printGoFile("main", map[string]string{
		"context":  "context",
		"gowdkapp": "gowdk-generated-app/gowdkapp",
		"log":      "log",
		"os":       "os",
		"signal":   "os/signal",
		"syscall":  "syscall",
	}, []ast.Decl{cronMainDecl()})
}

func cronMainDecl() ast.Decl {
	stmts := signalContextStmts()
	stmts = append(stmts,
		define([]ast.Expr{id("err")}, call(sel("gowdkapp", "RunContractCron"), id("ctx"))),
		returnIfRunErrStmt(),
	)
	return funcDecl("main", nil, nil, stmts)
}

func uniqueCronRunFunc(contractName string, used map[string]bool) string {
	base := "runContractJob" + source.ExportedIdentifier(contractTypeIdent(contractName), "Job")
	name := base
	for index := 2; used[name]; index++ {
		name = fmt.Sprintf("%s%d", base, index)
	}
	used[name] = true
	return name
}

func roleProviderAliases(refs ...gowdk.ServiceRef) map[string]string {
	paths := map[string]bool{}
	preferred := map[string]string{}
	for _, ref := range refs {
		if !serviceRefConfigured(ref) {
			continue
		}
		importPath := strings.TrimSpace(ref.ImportPath)
		paths[importPath] = true
		preferred[importPath] = path.Base(importPath)
	}
	return roleImportAliases(paths, preferred)
}

func serviceRefConfigured(ref gowdk.ServiceRef) bool {
	return strings.TrimSpace(ref.ImportPath) != "" || strings.TrimSpace(ref.Function) != ""
}

func validateOptionalRoleProvider(label string, ref gowdk.ServiceRef) error {
	if !serviceRefConfigured(ref) {
		return nil
	}
	return validateRoleProvider(label, ref)
}

func validateRoleProvider(label string, ref gowdk.ServiceRef) error {
	if strings.TrimSpace(ref.ImportPath) == "" {
		return fmt.Errorf("%s.ImportPath is required", label)
	}
	if strings.TrimSpace(ref.Function) == "" {
		return fmt.Errorf("%s.Function is required", label)
	}
	return nil
}
