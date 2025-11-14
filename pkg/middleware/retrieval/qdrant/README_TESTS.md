# Qdrant Integration Tests

## Overview

The Qdrant package includes comprehensive integration tests that use [testcontainers-go](https://golang.testcontainers.org/) to spin up real Qdrant instances for testing.

## Running Tests

### Run All Tests (Unit + Integration)
```bash
go test -tags=integration -v ./pkg/middleware/retrieval/qdrant/
```

### Run Only Integration Tests
```bash
go test -tags=integration -v -run="Test.*" ./pkg/middleware/retrieval/qdrant/
```

### Run Only Unit Tests
```bash
go test -v ./pkg/middleware/retrieval/qdrant/
```

### Skip Integration Tests in CI (if needed)
```bash
go test -short ./pkg/middleware/retrieval/qdrant/
```

## Test Coverage

### Integration Tests (`integration_test.go`)
- **TestClientCreation**: Client configuration, URL parsing, validation
- **TestHealthCheck**: Connection health checks, error handling  
- **TestStoreOperations**: Document storage (single, batch, large batches)
- **TestSearchOperations**: Vector search, filters, thresholds, limits
- **TestDeleteOperations**: Document deletion (single, batch, non-existent IDs)
- **TestSearchWithDiversification**: Hybrid search and RRF
- **TestEmbeddingProvider**: Embedding generation and provider management

### Unit Tests (`qdrant_test.go`)
- Payload building and conversion
- Filter construction
- Timestamp extraction
- Configuration validation

## Requirements

- **Docker**: Tests use testcontainers which requires Docker to be running
- **Go 1.24+**: Build tag support
- **Network**: Tests pull the `qdrant/qdrant:latest` image

## GitHub Actions

These tests work in GitHub Actions out of the box:

```yaml
- name: Run integration tests
  run: go test -tags=integration -v ./pkg/middleware/retrieval/qdrant/...
```

## Notes

- Tests are isolated using separate collections
- Each test suite spins up its own Qdrant container
- Mock embedding provider generates deterministic 128-dimensional vectors
- Tests include both positive and negative test cases
- All table-driven for clarity and maintainability
