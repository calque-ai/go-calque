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

	pipe.Use(flow.Logger("STEP1", 100)) // Add logging middleware

	// Test with JSON converter using new API
	jsonMap := map[string]any{"key": "value", "number": 42}
	var jsonResult string
	err := pipe.Run(context.Background(), convert.Json(jsonMap), &jsonResult)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("JSON result:")
	fmt.Println(jsonResult)
}
