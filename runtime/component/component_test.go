package component

import (
	"context"
	"errors"
	"testing"
)

func TestFuncRenderCallsWrappedFunction(t *testing.T) {
	component := Func(func(context.Context) (string, error) {
		return "<h1>Hello</h1>", nil
	})

	html, err := component.Render(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if html != "<h1>Hello</h1>" {
		t.Fatalf("unexpected html: %q", html)
	}
}

func TestFuncRenderPropagatesErrors(t *testing.T) {
	expected := errors.New("render failed")
	component := Func(func(context.Context) (string, error) {
		return "", expected
	})

	_, err := component.Render(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("expected render error, got %v", err)
	}
}
