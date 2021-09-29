package mtls

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

type noopLoader struct {
	name string
	CredentialsLoader
}

func TestRegister(t *testing.T) {
	unregisterAll()
	Register("foo", noopLoader{name: "foo"})
	Register("bar", noopLoader{name: "bar"})
	if diff := cmp.Diff([]string{"bar", "foo"}, Loaders()); diff != "" {
		t.Errorf("Loaders() mismatch (-want, +got):\n%s", diff)
	}
	for _, name := range []string{"foo", "bar"} {
		l, err := Loader(name)
		if err != nil {
			t.Errorf("Loader(%s) err was %v, want nil", name, err)
		}
		got := l.(noopLoader).name
		if got != name {
			t.Errorf("Loader(%s) returned loader with name %s, want %s", name, got, name)
		}
	}
}
