package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/calque-ai/calque-pipe/core"
)

func main() {
	flow := core.New()

	flow.
		Use(core.Logger("STEP1")).            // Add logging middleware
		Use(core.Transform(strings.ToUpper)). // Add transformation to uppercase
		Use(core.Branch(                      // Add conditional branching
			func(s string) bool { return strings.Contains(s, "HELLO") },
			core.Transform(func(s string) string { return s + " [GREETING DETECTED]" }),
			core.Transform(func(s string) string { return s + " [NO GREETING]" }),
		)).
		Use(core.Parallel( // Add parallel processing
			core.Transform(func(s string) string { return "Length: " + fmt.Sprint(len(s)) }),
			core.Transform(func(s string) string { return "Words: " + fmt.Sprint(len(strings.Fields(s))) }),
			core.Transform(func(s string) string { return "Reversed: " + reverse(s) }),
		))

	// Run the flow
	result, err := flow.Run(context.Background(), "hello world")
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
