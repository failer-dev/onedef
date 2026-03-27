package main

import (
	"fmt"

	"github.com/failer-dev/onedef"
)

func main() {
	app := onedef.New()
	app.Register(&GetUser{}, &CreateUser{})

	fmt.Println("listening on :8080")
	if err := app.Run(":8081"); err != nil {
		panic(err)
	}
}
