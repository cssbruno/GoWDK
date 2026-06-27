package app

import "testing"

func TestInheritedListenerWithoutEnv(t *testing.T) {
	t.Setenv("GOWDK_LISTENER_FD", "")
	listener, err := inheritedListener()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listener != nil {
		_ = listener.Close()
		t.Fatal("expected no inherited listener when GOWDK_LISTENER_FD is unset")
	}
}

func TestInheritedListenerInvalidFD(t *testing.T) {
	for _, value := range []string{"not-a-number", "-1"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("GOWDK_LISTENER_FD", value)
			listener, err := inheritedListener()
			if err == nil {
				if listener != nil {
					_ = listener.Close()
				}
				t.Fatalf("expected error for GOWDK_LISTENER_FD=%q", value)
			}
		})
	}
}
