package calque_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ctrl"
	"github.com/calque-ai/go-calque/pkg/middleware/text"
)

const testInputData = "test input data"

// Baseline benchmarks - raw function call costs
func BenchmarkBaseline_DirectFunctionCall(b *testing.B) {
	fn := func(s string) string { return s }
	input := "test"
	for i := 0; i < b.N; i++ {
		_ = fn(input)
	}
}

func BenchmarkBaseline_DirectChain3(b *testing.B) {
	fns := []func(string) string{
		func(s string) string { return s },
		func(s string) string { return s },
		func(s string) string { return s },
	}
	input := "test"
	for i := 0; i < b.N; i++ {
		result := input
		for _, fn := range fns {
			result = fn(result)
		}
	}
}

// Flow setup benchmarks - cost of creating flows
func BenchmarkFlowSetup_NewFlow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = calque.NewFlow()
	}
}

func BenchmarkFlowSetup_NewFlowWithConfig(b *testing.B) {
	config := calque.FlowConfig{MaxConcurrent: 10}
	for i := 0; i < b.N; i++ {
		_ = calque.NewFlow(config)
	}
}

func BenchmarkFlowSetup_AddHandlers(b *testing.B) {
	handler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		_, err := io.Copy(w.Data, r.Data)
		return err
	})
	for i := 0; i < b.N; i++ {
		_ = calque.NewFlow().Use(handler).Use(handler).Use(handler)
	}
}

// Single handler benchmarks - overhead of one handler
func BenchmarkSingleHandler_Passthrough(b *testing.B) {
	flow := calque.NewFlow().Use(calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		_, err := io.Copy(w.Data, r.Data)
		return err
	}))

	ctx := context.Background()
	input := testInputData

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output string
		_ = flow.Run(ctx, input, &output)
	}
}

func BenchmarkSingleHandler_Transform(b *testing.B) {
	flow := calque.NewFlow().Use(calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input string
		if err := calque.Read(r, &input); err != nil {
			return err
		}
		return calque.Write(w, strings.ToUpper(input))
	}))

	ctx := context.Background()
	input := testInputData

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output string
		_ = flow.Run(ctx, input, &output)
	}
}

func BenchmarkSingleHandler_TextTransform(b *testing.B) {
	flow := calque.NewFlow().Use(text.Transform(strings.ToUpper))

	ctx := context.Background()
	input := testInputData

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output string
		_ = flow.Run(ctx, input, &output)
	}
}

// Multi-handler scaling - how overhead grows with handlers
func benchmarkMultiHandler(b *testing.B, count int) {
	handler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		_, err := io.Copy(w.Data, r.Data)
		return err
	})

	flow := calque.NewFlow()
	for i := 0; i < count; i++ {
		flow = flow.Use(handler)
	}

	ctx := context.Background()
	input := testInputData

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output string
		_ = flow.Run(ctx, input, &output)
	}
}

func BenchmarkMultiHandler_1(b *testing.B)  { benchmarkMultiHandler(b, 1) }
func BenchmarkMultiHandler_3(b *testing.B)  { benchmarkMultiHandler(b, 3) }
func BenchmarkMultiHandler_5(b *testing.B)  { benchmarkMultiHandler(b, 5) }
func BenchmarkMultiHandler_10(b *testing.B) { benchmarkMultiHandler(b, 10) }

// Chain vs Flow - sequential vs concurrent comparison
func BenchmarkChainVsFlow_Chain3(b *testing.B) {
	handler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input string
		if err := calque.Read(r, &input); err != nil {
			return err
		}
		return calque.Write(w, input)
	})

	flow := calque.NewFlow().Use(ctrl.Chain(handler, handler, handler))

	ctx := context.Background()
	input := testInputData

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output string
		_ = flow.Run(ctx, input, &output)
	}
}

func BenchmarkChainVsFlow_Flow3(b *testing.B) {
	handler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		_, err := io.Copy(w.Data, r.Data)
		return err
	})

	flow := calque.NewFlow().Use(handler).Use(handler).Use(handler)

	ctx := context.Background()
	input := testInputData

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output string
		_ = flow.Run(ctx, input, &output)
	}
}

// AI latency simulation - framework overhead vs AI time
func benchmarkAILatency(b *testing.B, delay time.Duration) {
	handler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input string
		if err := calque.Read(r, &input); err != nil {
			return err
		}
		time.Sleep(delay) // Simulate AI latency
		return calque.Write(w, "AI response: "+input)
	})

	flow := calque.NewFlow().Use(handler)
	ctx := context.Background()
	input := "test prompt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output string
		_ = flow.Run(ctx, input, &output)
	}
}

func BenchmarkAILatency_1ms(b *testing.B)   { benchmarkAILatency(b, 1*time.Millisecond) }
func BenchmarkAILatency_10ms(b *testing.B)  { benchmarkAILatency(b, 10*time.Millisecond) }
func BenchmarkAILatency_100ms(b *testing.B) { benchmarkAILatency(b, 100*time.Millisecond) }

// Memory benchmarks - allocation patterns
func BenchmarkMemory_NoHandlers(b *testing.B) {
	for i := 0; i < b.N; i++ {
		flow := calque.NewFlow()
		ctx := context.Background()
		var output string
		_ = flow.Run(ctx, "test", &output)
	}
}

func BenchmarkMemory_SingleHandler(b *testing.B) {
	handler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		_, err := io.Copy(w.Data, r.Data)
		return err
	})

	for i := 0; i < b.N; i++ {
		flow := calque.NewFlow().Use(handler)
		ctx := context.Background()
		var output string
		_ = flow.Run(ctx, "test", &output)
	}
}

func BenchmarkMemory_5Handlers(b *testing.B) {
	handler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		_, err := io.Copy(w.Data, r.Data)
		return err
	})

	for i := 0; i < b.N; i++ {
		flow := calque.NewFlow().Use(handler).Use(handler).Use(handler).Use(handler).Use(handler)
		ctx := context.Background()
		var output string
		_ = flow.Run(ctx, "test", &output)
	}
}

// Goroutine and pipe overhead - low-level costs
func BenchmarkGoroutine_Overhead(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			wg.Done()
		}()
		wg.Wait()
	}
}

func BenchmarkGoroutine_WithPipe(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pr, pw := io.Pipe()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			pw.Write([]byte("test"))
			pw.Close()
			wg.Done()
		}()
		io.ReadAll(pr)
		wg.Wait()
	}
}

// Concurrent execution - multiple parallel flows
func benchmarkConcurrentFlows(b *testing.B, numFlows int) {
	handler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		_, err := io.Copy(w.Data, r.Data)
		return err
	})

	flow := calque.NewFlow().Use(handler)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for j := 0; j < numFlows; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var output string
				_ = flow.Run(ctx, "test", &output)
			}()
		}
		wg.Wait()
	}
}

func BenchmarkConcurrent_10Flows(b *testing.B)  { benchmarkConcurrentFlows(b, 10) }
func BenchmarkConcurrent_100Flows(b *testing.B) { benchmarkConcurrentFlows(b, 100) }

// Data size scaling - throughput at different sizes
func benchmarkDataSize(b *testing.B, size int) {
	handler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		_, err := io.Copy(w.Data, r.Data)
		return err
	})

	flow := calque.NewFlow().Use(handler)
	ctx := context.Background()
	input := strings.Repeat("x", size)

	b.ResetTimer()
	b.SetBytes(int64(size))
	for i := 0; i < b.N; i++ {
		var output string
		_ = flow.Run(ctx, input, &output)
	}
}

func BenchmarkDataSize_100B(b *testing.B)  { benchmarkDataSize(b, 100) }
func BenchmarkDataSize_1KB(b *testing.B)   { benchmarkDataSize(b, 1024) }
func BenchmarkDataSize_10KB(b *testing.B)  { benchmarkDataSize(b, 10*1024) }
func BenchmarkDataSize_100KB(b *testing.B) { benchmarkDataSize(b, 100*1024) }

// Streaming vs buffered - io.Copy vs io.ReadAll
func BenchmarkStreaming_IOCopy(b *testing.B) {
	data := []byte(strings.Repeat("x", 10*1024))
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		src := bytes.NewReader(data)
		dst := &bytes.Buffer{}
		io.Copy(dst, src)
	}
}

func BenchmarkStreaming_ReadAll(b *testing.B) {
	data := []byte(strings.Repeat("x", 10*1024))
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		src := bytes.NewReader(data)
		result, _ := io.ReadAll(src)
		dst := &bytes.Buffer{}
		dst.Write(result)
	}
}

// Realistic AI workflows - simulated agent patterns
func BenchmarkRealisticWorkflow_SimpleChat(b *testing.B) {
	// Simulates: prompt formatting -> AI call (1ms) -> response formatting
	promptHandler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input string
		if err := calque.Read(r, &input); err != nil {
			return err
		}
		return calque.Write(w, "User: "+input+"\nAssistant: ")
	})

	aiHandler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input string
		if err := calque.Read(r, &input); err != nil {
			return err
		}
		time.Sleep(1 * time.Millisecond) // Simulate AI
		return calque.Write(w, input+"This is the AI response.")
	})

	postHandler := calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		var input string
		if err := calque.Read(r, &input); err != nil {
			return err
		}
		return calque.Write(w, strings.TrimSpace(input))
	})

	flow := calque.NewFlow().Use(ctrl.Chain(promptHandler, aiHandler, postHandler))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output string
		_ = flow.Run(ctx, "Hello!", &output)
	}
}
