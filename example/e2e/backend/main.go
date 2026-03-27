package main

import (
	"github.com/failer-dev/onedef"
)

func main() {
	app := onedef.New()
	app.Register(&GetUser{}, &CreateUser{})

	if err := app.Run(":8081"); err != nil {
		panic(err)
	}
}
