// Package memory provides memory middleware with gRPC support for distributed memory services.
package memory

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/calque-ai/go-calque/pkg/helpers"
	calquepb "github.com/calque-ai/go-calque/proto"
)

// GRPCStore implements the Store interface for remote gRPC memory services.
//
// Provides distributed memory capabilities by connecting to remote gRPC memory services.
// Supports context and conversation memory with protobuf serialization.
//
// Example:
//
//	store, _ := memory.NewGRPCStore("memory-service:8080")
//	ctx := memory.NewContext(store)
type GRPCStore struct {
	conn   *grpc.ClientConn
	client calquepb.MemoryServiceClient
}

// NewGRPCStore creates a new gRPC memory store.
func NewGRPCStore(endpoint string) (*GRPCStore, error) {
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, helpers.WrapGRPCError(err, "failed to connect to memory service", endpoint)
	}

	client := calquepb.NewMemoryServiceClient(conn)
	return &GRPCStore{
		conn:   conn,
		client: client,
	}, nil
}

// createContext creates a context with timeout for gRPC calls
func (s *GRPCStore) createContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// Get retrieves a value from the remote memory store.
func (s *GRPCStore) Get(key string) ([]byte, error) {
	ctx, cancel := s.createContext()
	defer cancel()

	req := &calquepb.MemoryRequest{
		Operation: "get",
		Key:       key,
	}

	resp, err := s.client.ProcessMemory(ctx, req)
	if err != nil {
		return nil, helpers.WrapGRPCError(err, "failed to get value for key", key)
	}

	if !resp.Success {
		return nil, helpers.NewGRPCInternalError("memory operation failed", nil)
	}

	return []byte(resp.Value), nil
}

// Set stores a value in the remote memory store.
func (s *GRPCStore) Set(key string, value []byte) error {
	ctx, cancel := s.createContext()
	defer cancel()

	req := &calquepb.MemoryRequest{
		Operation: "set",
		Key:       key,
		Value:     string(value),
	}

	resp, err := s.client.ProcessMemory(ctx, req)
	if err != nil {
		return helpers.WrapGRPCError(err, "failed to set value for key", key)
	}

	if !resp.Success {
		return helpers.NewGRPCInternalError("memory operation failed", nil)
	}

	return nil
}

// Delete removes a value from the remote memory store.
func (s *GRPCStore) Delete(key string) error {
	ctx, cancel := s.createContext()
	defer cancel()

	req := &calquepb.MemoryRequest{
		Operation: "delete",
		Key:       key,
	}

	resp, err := s.client.ProcessMemory(ctx, req)
	if err != nil {
		return helpers.WrapGRPCError(err, "failed to delete key", key)
	}

	if !resp.Success {
		return helpers.NewGRPCInternalError("memory operation failed", nil)
	}

	return nil
}

// List returns all keys in the remote memory store.
func (s *GRPCStore) List() []string {
	ctx, cancel := s.createContext()
	defer cancel()

	req := &calquepb.MemoryRequest{
		Operation: "list",
	}

	resp, err := s.client.ProcessMemory(ctx, req)
	if err != nil {
		return []string{} // Return empty list on error
	}

	if !resp.Success {
		return []string{} // Return empty list on failure
	}

	// Parse the response to extract keys
	// The keys are returned as a comma-separated string in the value field
	if resp.Value == "" {
		return []string{}
	}

	// Split the comma-separated keys
	keys := strings.Split(resp.Value, ",")

	// Trim whitespace from each key
	for i, key := range keys {
		keys[i] = strings.TrimSpace(key)
	}

	return keys
}

// Exists checks if a key exists in the remote memory store.
func (s *GRPCStore) Exists(key string) bool {
	ctx, cancel := s.createContext()
	defer cancel()

	req := &calquepb.MemoryRequest{
		Operation: "exists",
		Key:       key,
	}

	resp, err := s.client.ProcessMemory(ctx, req)
	if err != nil {
		return false // Return false on error
	}

	return resp.Success
}

// Close closes the gRPC connection.
func (s *GRPCStore) Close() error {
	return s.conn.Close()
}
