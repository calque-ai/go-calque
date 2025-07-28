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

	// Test the new API with simple built-in types
	var result string
	err := pipe.Run(context.Background(), "hello world", &result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("String result:")
	fmt.Println(result)

	// Test with []byte output
	var byteResult []byte
	err = pipe.Run(context.Background(), "hello bytes", &byteResult)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Byte result:")
	fmt.Println(string(byteResult))

	// Test with JSON converter using new API
	jsonMap := map[string]any{"key": "value", "number": 42}
	var jsonResult string
	err = pipe.Run(context.Background(), convert.Json(jsonMap), &jsonResult)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("JSON result:")
	fmt.Println(jsonResult)
}
