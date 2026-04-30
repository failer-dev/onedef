package main

import (
	"fmt"
	"log"

	"github.com/failer-dev/onedef/example/chat/backend/internal/api"
	"github.com/failer-dev/onedef/onedef_go"
)

func main() {
	specJSON, err := api.APISpec().GenerateIRJSON(onedef.GenerateIROptions{
		Initialisms: []string{"ID"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(specJSON))
}
