# Go Package Review Guidelines

## Package-Specific Rules

### Controller Package (`pkg/controller/`)
- Ensure proper reconciliation logic
- Check for proper error handling in Reconcile function
- Validate proper use of controller-runtime patterns
- Ensure proper finalizer management
- Check for proper event recording

### Kubernetes Client Package (`pkg/kube/`)
- Validate proper client initialization
- Check for proper error handling in client operations
- Ensure proper context usage
- Validate proper resource version handling
- Check for proper mocking in tests

### Snapshots Package (`pkg/snapshots/`)
- Validate snapshot creation and deletion logic
- Check for proper error handling in snapshot operations
- Ensure proper cleanup of failed snapshots
- Validate snapshot group management
- Check for proper validation of snapshot states

### Types Package (`pkg/types/`)
- Validate CRD schema definitions
- Check for proper versioning strategy
- Ensure proper validation logic
- Validate webhook configurations
- Check for proper serialization/deserialization

## Go-Specific Patterns

### Error Handling
- Use proper error wrapping with `fmt.Errorf`
- Implement custom error types where appropriate
- Check for proper error propagation
- Validate error handling in all exported functions

### Testing
- All exported functions must have unit tests
- Controller logic must have integration tests
- Use proper mocking for external dependencies
- Test both success and error paths
- Use table-driven tests for multiple scenarios

### Performance
- Check for proper resource usage
- Validate efficient use of Kubernetes API calls
- Ensure proper caching strategies
- Check for memory leaks in long-running operations

### Security
- Validate all inputs from external sources
- Check for proper authentication and authorization
- Ensure no sensitive data is logged
- Validate proper context usage for cancellation
