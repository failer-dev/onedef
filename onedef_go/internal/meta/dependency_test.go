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

func TestNewProvideBinding_ConcreteAndInterfaceTypes(t *testing.T) {
	t.Parallel()

	repo := &bindingRepo{}

	concrete := NewProvideBinding(repo)
	if got := ProvideType(concrete); got != reflect.TypeOf(repo) {
		t.Fatalf("concrete type = %v, want %v", got, reflect.TypeOf(repo))
	}

	contract := NewProvideBinding[bindingTestRepo](repo)
	if got := ProvideType(contract); got != reflect.TypeFor[bindingTestRepo]() {
		t.Fatalf("interface type = %v, want %v", got, reflect.TypeFor[bindingTestRepo]())
	}
	if got := ProvideValue(contract).Interface().(bindingTestRepo).Find(); got != "ok" {
		t.Fatalf("interface value Find() = %q, want %q", got, "ok")
	}
}

func TestNewProvideBinding_PanicsForNil(t *testing.T) {
	t.Parallel()

	assertBindingPanicContains(t, func() {
		NewProvideBinding[*bindingRepo](nil)
	}, "cannot provide nil value")

	assertBindingPanicContains(t, func() {
		NewProvideBinding[bindingTestRepo](nil)
	}, "cannot provide nil value")
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
