package meta

import (
	"reflect"
	"strings"
	"testing"
)

type bindingTestRepo interface {
	Find() string
}

type bindingRepo struct{}

func (bindingRepo) Find() string {
	return "ok"
}

func TestNewDependencyBinding_ConcreteAndInterfaceTypes(t *testing.T) {
	t.Parallel()

	repo := &bindingRepo{}

	concrete := NewDependencyBinding(repo)
	if got := DependencyType(concrete); got != reflect.TypeOf(repo) {
		t.Fatalf("concrete type = %v, want %v", got, reflect.TypeOf(repo))
	}

	contract := NewDependencyBinding[bindingTestRepo](repo)
	if got := DependencyType(contract); got != reflect.TypeFor[bindingTestRepo]() {
		t.Fatalf("interface type = %v, want %v", got, reflect.TypeFor[bindingTestRepo]())
	}
	if got := DependencyValue(contract).Interface().(bindingTestRepo).Find(); got != "ok" {
		t.Fatalf("interface value Find() = %q, want %q", got, "ok")
	}
}

func TestNewDependencyBinding_PanicsForNil(t *testing.T) {
	t.Parallel()

	assertBindingPanicContains(t, func() {
		NewDependencyBinding[*bindingRepo](nil)
	}, "cannot bind nil dependency")

	assertBindingPanicContains(t, func() {
		NewDependencyBinding[bindingTestRepo](nil)
	}, "cannot bind nil dependency")
}

func assertBindingPanicContains(t *testing.T, fn func(), want string) {
	t.Helper()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got nil")
		}
		if !strings.Contains(r.(string), want) {
			t.Fatalf("panic = %q, want substring %q", r, want)
		}
	}()

	fn()
}
