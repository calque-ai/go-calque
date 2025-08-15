package multiagent

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/convert"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
)

// routeHandler holds handler with its metadata
type routeHandler struct {
	name        string
	description string
	keywords    []string
	handler     calque.Handler
}

// ServeFlow implements the Handler interface for routeHandler
func (rh *routeHandler) ServeFlow(req *calque.Request, res *calque.Response) error {
	return rh.handler.ServeFlow(req, res)
}

// RouteSelection defines the structured output schema for route selection
type RouteSelection struct {
	Route      string  `json:"route" jsonschema:"required,description=Selected route identifier"`
	Confidence float64 `json:"confidence,omitempty" jsonschema:"minimum=0,maximum=1,description=Confidence score for the selection"`
	Reasoning  string  `json:"reasoning,omitempty" jsonschema:"description=Brief explanation for the route choice"`
}

// RouterInput contains the request data and available route options for the selector
type RouterInput struct {
	Request string        `json:"request" jsonschema:"required,description=The user request to route"`
	Routes  []RouteOption `json:"routes" jsonschema:"required,description=Available routing options"`
}

// RouteOption describes an available route for selection
type RouteOption struct {
	ID          string `json:"id" jsonschema:"required,description=Route identifier"`
	Name        string `json:"name" jsonschema:"required,description=Human readable route name"`
	Description string `json:"description" jsonschema:"required,description=What this route handles"`
	Keywords    string `json:"keywords,omitempty" jsonschema:"description=Relevant keywords for this route"`
}

// Route wraps any handler with routing metadata for intelligent selection.
// The handler can be a simple agent or a complex pipeline.
//
// Input: any data type (passes through to wrapped handler)
// Output: response from wrapped handler
// Behavior: STREAMING - metadata is stored, then delegates to handler
//
// Example:
//
//	mathHandler := multiagent.Route(ai.Agent(mathClient), "math", "Mathematical calculations", "calculate,solve,math")
//	codeHandler := multiagent.Route(
//	    calque.Flow().Use(tools.Registry(codeTools...)).Use(ai.Agent(codeClient)),
//	    "code", "Programming and debugging", "code,debug,program")
func Route(handler calque.Handler, name, description, keywords string) calque.Handler {
	return &routeHandler{
		name:        name,
		description: description,
		keywords:    strings.Split(keywords, ","),
		handler:     handler,
	}
}

// Router creates intelligent handler selection using structured JSON Schema output.
// The router takes an AI client and automatically configures it with the RouteSelection schema.
// No manual prompt setup is needed - the router generates structured input with route metadata.
//
// Input: any data type (buffered - needs full input for selection)
// Output: response from selected handler
// Behavior: BUFFERED - reads input, creates structured prompt with schema, validates response
//
// Example:
//
//	router := multiagent.Router(selectionClient,
//	    mathHandler, codeHandler, searchHandler)
func Router(client ai.Client, handlers ...calque.Handler) calque.Handler {
	if len(handlers) == 0 {
		return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
			return fmt.Errorf("no handlers provided to router")
		})
	}

	// Extract route metadata from wrapped handlers
	routes := make([]*routeHandler, len(handlers))
	routeOptions := make([]RouteOption, len(handlers))

	for i, h := range handlers {
		if rh, ok := h.(*routeHandler); ok {
			routes[i] = rh
			routeOptions[i] = RouteOption{
				ID:          rh.name,
				Name:        rh.name,
				Description: rh.description,
				Keywords:    strings.Join(rh.keywords, ","),
			}
		} else {
			// Handle non-routed handlers with default metadata
			routeName := fmt.Sprintf("handler_%d", i)
			routes[i] = &routeHandler{
				name:        routeName,
				description: fmt.Sprintf("Handler %d", i),
				handler:     h,
			}
			routeOptions[i] = RouteOption{
				ID:          routeName,
				Name:        routeName,
				Description: fmt.Sprintf("Handler %d", i),
			}
		}
	}

	// Create AI agent with schema for route selection
	selector := ai.Agent(client, ai.WithSchema(&RouteSelection{}))

	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input []byte
		err := calque.Read(req, &input)
		if err != nil {
			return err
		}

		// Create structured input with route options
		routerInput := RouterInput{
			Request: string(input),
			Routes:  routeOptions,
		}

		// Try selection with retry logic
		maxRetries := 2
		var selectedHandler calque.Handler

		for attempt := 0; attempt <= maxRetries; attempt++ {
			selection, err := callSelectorWithSchema(req.Context, selector, routerInput)

			if err == nil {
				// Validate the selected route exists
				if selectedHandler = findHandlerByID(selection.Route, routes); selectedHandler != nil {
					break
				}
			}

			if attempt == maxRetries {
				// Final fallback - use first handler
				selectedHandler = routes[0].handler
				break
			}
		}

		// Route to selected handler
		req.Data = bytes.NewReader(input)
		return selectedHandler.ServeFlow(req, res)
	})
}

// callSelectorWithSchema creates schema input, calls selector, and parses structured output
func callSelectorWithSchema(ctx context.Context, selector calque.Handler, routerInput RouterInput) (*RouteSelection, error) {
	// Create flow with schema converters - agent already has WithSchema
	flow := calque.NewFlow().Use(selector)

	var selection RouteSelection
	err := flow.Run(ctx, convert.ToJsonSchema(routerInput), convert.FromJson(&selection))
	if err != nil {
		return nil, fmt.Errorf("selector flow failed: %w", err)
	}

	// Validate required fields
	if selection.Route == "" {
		return nil, fmt.Errorf("selector output missing required 'route' field")
	}

	return &selection, nil
}

// findHandlerByID locates handler by route ID
func findHandlerByID(routeID string, routes []*routeHandler) calque.Handler {
	for _, route := range routes {
		if route.name == routeID {
			return route.handler
		}
	}
	return nil
}

// LoadBalancer distributes requests across handlers using round-robin.
// All handlers should be functionally equivalent for load distribution.
//
// Example:
//
//	lb := multiagent.LoadBalancer(agent1, agent2, agent3)
func LoadBalancer(handlers ...calque.Handler) calque.Handler {
	if len(handlers) == 0 {
		return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
			return fmt.Errorf("no handlers provided to load balancer")
		})
	}

	counter := 0
	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		handler := handlers[counter%len(handlers)]
		counter++
		return handler.ServeFlow(req, res)
	})
}
