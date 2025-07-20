package main

import (
	"context"
	"fmt"
	"log"

	"github.com/calque-ai/calque-pipe/convert"
	"github.com/calque-ai/calque-pipe/core"
)

func main() {
	flow := core.New()

	flow.
		Use(core.Logger("STEP1")) // Add logging middleware

	// Run the flow
	jsonMap := map[string]any{"key": "value"}
	result, err := flow.Run(context.Background(), jsonMap, convert.JSON)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Final result:")
	fmt.Println(result)
}
