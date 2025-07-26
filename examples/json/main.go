package main

import (
	"context"
	"fmt"
	"log"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/converter"
	"github.com/calque-ai/calque-pipe/middleware/flow"
)

func main() {
	pipe := core.New()

	pipe.
		Use(flow.Logger("STEP1", 100)). // Add logging middleware
		Use(converter.JsonInput()).     // Process JSON input 
		Use(converter.JsonOutput())     // Format JSON output

	// Run the pipe - no converter needed, handled by middleware
	jsonMap := map[string]any{"key": "value"}
	result, err := pipe.Run(context.Background(), jsonMap)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Final result:")
	fmt.Println(result)
}
