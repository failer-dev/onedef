package main

import (
	"log"
	"os"

	"github.com/failer-dev/onedef/example/chat/backend/internal/api"
	"github.com/failer-dev/onedef/onedef_go"
)

func main() {
	addr := ":8280"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	app := onedef.New(api.APISpec())
	if err := app.Run(addr); err != nil {
		log.Fatal(err)
	}
}
