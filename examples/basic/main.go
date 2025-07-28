package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/flow"
	str "github.com/calque-ai/calque-pipe/middleware/strings"
)

func main() {
	pipe := core.New()

	pipe.
		Use(flow.Logger("STEP1", 100)).      // Add logging middleware
		Use(str.Transform(strings.ToUpper)). // Add transformation to uppercase
		Use(str.Branch(                      // Add conditional branching
			func(s string) bool { return strings.Contains(s, "HELLO") },
			str.Transform(func(s string) string { return s + " [GREETING DETECTED]" }),
			str.Transform(func(s string) string { return s + " [NO GREETING]" }),
		)).
		Use(flow.Parallel[string]( // Add parallel processing
			str.Transform(func(s string) string { return "Length: " + fmt.Sprint(len(s)) }),
			str.Transform(func(s string) string { return "Words: " + fmt.Sprint(len(strings.Fields(s))) }),
			str.Transform(func(s string) string { return "Reversed: " + reverse(s) }),
		))

	// Run the pipe
	var result string
	err := pipe.Run(context.Background(), "hello world", &result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Final result:")
	fmt.Println(result)
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
