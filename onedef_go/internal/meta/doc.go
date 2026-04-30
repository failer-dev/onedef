// Package meta defines onedef's internal endpoint definition model.
//
// The package is shared by the public DSL, reflection inspectors, the runtime
// app, and IR generation. Its types are exported for internal package
// boundaries, but most values are expected to be created through constructor
// functions so validation happens before the app consumes them.
package meta
