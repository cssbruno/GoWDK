package cssscope

import "testing"

func TestScopeIDIsStableForHashKey(t *testing.T) {
	key := HashKey("component", "ui", "Hero", "components/hero.cmp.gwdk", "./hero.css")

	if key != "component:ui:Hero:components/hero.cmp.gwdk:./hero.css" {
		t.Fatalf("unexpected hash key: %q", key)
	}
	if got, want := ScopeID(key), "gwdk-e3110764ef6b"; got != want {
		t.Fatalf("unexpected scope id: got %q want %q", got, want)
	}
}
