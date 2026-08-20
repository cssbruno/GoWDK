package gowdk

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
	gowdkguard "github.com/cssbruno/gowdk/runtime/guard"
)

// InteropConfig explicitly connects generated runtime integration points to
// ordinary application Go packages. Registration constructors accept real Go
// functions so rename and find-references tooling can follow each binding.
type InteropConfig struct {
	Loads        []LoadRegistration
	Guards       GuardRegistration
	AuthProvider AuthProviderRegistration
}

// GoHookRef is compiler metadata derived from a typed Go function value.
// Applications should create refs through the Register* constructors.
type GoHookRef struct {
	ImportPath string
	Function   string
	SourceFile string
}

// LoadRegistration binds one page ID to its request-time load function.
type LoadRegistration struct {
	Page string
	Hook GoHookRef
	err  string
}

// GuardRegistration binds generated route startup to a guard registry provider.
type GuardRegistration struct {
	Hook GoHookRef
	err  string
}

// AuthProviderRegistration binds native RBAC guards to an auth provider factory.
type AuthProviderRegistration struct {
	Hook GoHookRef
	err  string
}

// RegisterLoad explicitly binds a page server block to an exported Go load
// function. The compiler validates the supported ssr.LoadContext signature.
func RegisterLoad(page string, load any) LoadRegistration {
	ref, err := hookRef(load)
	registration := LoadRegistration{Page: strings.TrimSpace(page), Hook: ref}
	if err != nil {
		registration.err = err.Error()
	}
	return registration
}

// RegisterGuards explicitly binds custom guards to a typed registry provider.
func RegisterGuards(provider func() gowdkguard.Registry) GuardRegistration {
	ref, err := hookRef(provider)
	registration := GuardRegistration{Hook: ref}
	if err != nil {
		registration.err = err.Error()
	}
	return registration
}

// RegisterAuthProvider explicitly binds native RBAC to a typed provider factory.
func RegisterAuthProvider(provider func() gowdkauth.Provider) AuthProviderRegistration {
	ref, err := hookRef(provider)
	registration := AuthProviderRegistration{Hook: ref}
	if err != nil {
		registration.err = err.Error()
	}
	return registration
}

func hookRef(function any) (GoHookRef, error) {
	value := reflect.ValueOf(function)
	if !value.IsValid() || value.Kind() != reflect.Func || value.IsNil() {
		return GoHookRef{}, fmt.Errorf("registration requires a non-nil package-level function")
	}
	resolved := runtime.FuncForPC(value.Pointer())
	if resolved == nil {
		return GoHookRef{}, fmt.Errorf("registration function could not be resolved")
	}
	fullName := resolved.Name()
	dot := strings.LastIndex(fullName, ".")
	if dot <= strings.LastIndex(fullName, "/") || dot == len(fullName)-1 {
		return GoHookRef{}, fmt.Errorf("registration requires an exported package-level function")
	}
	functionName := fullName[dot+1:]
	if !exportedIdentifier(functionName) || strings.ContainsAny(functionName, ".[]") {
		return GoHookRef{}, fmt.Errorf("registration function %q must be an exported, non-generic package-level function", fullName)
	}
	file, _ := resolved.FileLine(value.Pointer())
	return GoHookRef{ImportPath: fullName[:dot], Function: functionName, SourceFile: file}, nil
}

func exportedIdentifier(value string) bool {
	if value == "" {
		return false
	}
	first := rune(value[0])
	return first >= 'A' && first <= 'Z'
}

// Validate rejects incomplete, duplicate, and constructor-invalid bindings.
func (config InteropConfig) Validate() error {
	seenPages := map[string]bool{}
	for index, registration := range config.Loads {
		if registration.err != "" {
			return fmt.Errorf("Interop.Loads[%d]: %s", index, registration.err)
		}
		page := strings.TrimSpace(registration.Page)
		if page == "" {
			return fmt.Errorf("Interop.Loads[%d].Page is required", index)
		}
		if seenPages[page] {
			return fmt.Errorf("Interop.Loads[%d].Page %q is registered more than once", index, page)
		}
		seenPages[page] = true
		if err := registration.Hook.validate(fmt.Sprintf("Interop.Loads[%d].Hook", index)); err != nil {
			return err
		}
	}
	if config.Guards.err != "" {
		return fmt.Errorf("Interop.Guards: %s", config.Guards.err)
	}
	if !config.Guards.Hook.empty() {
		if err := config.Guards.Hook.validate("Interop.Guards.Hook"); err != nil {
			return err
		}
	}
	if config.AuthProvider.err != "" {
		return fmt.Errorf("Interop.AuthProvider: %s", config.AuthProvider.err)
	}
	if !config.AuthProvider.Hook.empty() {
		if err := config.AuthProvider.Hook.validate("Interop.AuthProvider.Hook"); err != nil {
			return err
		}
	}
	return nil
}

func (ref GoHookRef) empty() bool {
	return strings.TrimSpace(ref.ImportPath) == "" && strings.TrimSpace(ref.Function) == "" && strings.TrimSpace(ref.SourceFile) == ""
}

func (ref GoHookRef) validate(path string) error {
	if strings.TrimSpace(ref.ImportPath) == "" || strings.TrimSpace(ref.Function) == "" || strings.TrimSpace(ref.SourceFile) == "" {
		return fmt.Errorf("%s must be created by a typed Register* constructor", path)
	}
	return nil
}

// LoadForPage returns the explicit request-time load binding for page, if any.
func (config InteropConfig) LoadForPage(page string) (LoadRegistration, bool) {
	for _, registration := range config.Loads {
		if registration.Page == page {
			return registration, true
		}
	}
	return LoadRegistration{}, false
}

// Configured reports whether a guard provider was explicitly registered.
func (registration GuardRegistration) Configured() bool { return !registration.Hook.empty() }

// Configured reports whether an auth provider was explicitly registered.
func (registration AuthProviderRegistration) Configured() bool { return !registration.Hook.empty() }
