package main

import (
	"context"
	"fmt"
	"log"

	"github.com/calque-ai/calque-pipe/convert"
	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/flow"
)

func main() {
	pipe := core.New()

	pipe.
		Use(flow.Logger("STEP1")) // Add logging middleware

	// Run the pipe
	jsonMap := map[string]any{"key": "value"}
	result, err := pipe.Run(context.Background(), jsonMap, convert.JSON)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Final result:")
	fmt.Println(result)
}
