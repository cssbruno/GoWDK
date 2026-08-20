package interop

import (
	"net/http"

	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
	gowdkguard "github.com/cssbruno/gowdk/runtime/guard"
	"github.com/cssbruno/gowdk/runtime/ssr"
)

func LoadDashboard(ssr.LoadContext) (map[string]any, error) {
	return map[string]any{"title": "Dashboard"}, nil
}

func Guards() gowdkguard.Registry {
	return gowdkguard.Registry{
		"session":       func(gowdkguard.Context) error { return nil },
		"auth.required": func(gowdkguard.Context) error { return nil },
	}
}

func AuthProvider() gowdkauth.Provider {
	return gowdkauth.ProviderFunc(func(*http.Request) (*gowdkauth.Principal, error) {
		return &gowdkauth.Principal{ID: "test-user"}, nil
	})
}
