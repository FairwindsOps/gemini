# Gemini Project Review Guidelines

## Project Overview
Gemini is a Kubernetes controller for managing volume snapshots. This project uses Go with Kubernetes client-go and controller-runtime.

## Security Focus Areas

### Kubernetes Security
- Validate all Kubernetes resource inputs before processing
- Check for proper RBAC permissions in controller code
- Ensure proper error handling for Kubernetes API calls
- Validate CRD (Custom Resource Definition) schemas
- Check for proper namespace isolation in multi-tenant scenarios

### Go Security
- Validate all user inputs and external data
- Check for proper error handling in all functions
- Ensure no sensitive data is logged or exposed
- Validate file paths and prevent path traversal attacks
- Check for proper context usage in goroutines

### API Security
- Validate all API request parameters
- Ensure proper authentication and authorization checks
- Check for proper input sanitization
- Validate JSON/YAML parsing with proper error handling

## Architecture Patterns

### Controller Patterns
- Use dependency injection for Kubernetes clients
- Follow the controller-runtime patterns consistently
- Implement proper reconciliation loops
- Use proper finalizers for resource cleanup
- Follow the operator pattern for CRD management

### Go Best Practices
- Use proper error wrapping with `fmt.Errorf` and `errors.Wrap`
- Implement proper logging with structured logging
- Use context.Context for cancellation and timeouts
- Follow Go naming conventions (camelCase for functions, PascalCase for exported)
- Use proper package organization and imports

### Kubernetes Patterns
- Use proper resource versioning for optimistic concurrency
- Implement proper retry logic for transient failures
- Use proper event recording for debugging
- Follow Kubernetes API conventions

## Common Issues to Check

### Go Code Quality
- Memory leaks in goroutines (check for proper cleanup)
- Missing error handling in critical paths
- Inconsistent naming conventions
- Missing or incorrect comments for exported functions
- Improper use of pointers vs values
- Missing context cancellation in long-running operations

### Kubernetes Controller Issues
- Missing finalizers for proper cleanup
- Improper resource version handling
- Missing proper error handling for API calls
- Inconsistent event recording
- Missing proper validation in webhooks
- Improper use of informers and listers

### Testing Issues
- Missing unit tests for critical functions
- Missing integration tests for controller logic
- Improper mocking of Kubernetes clients
- Missing test coverage for error paths
- Inconsistent test naming conventions

## Code Style Guidelines

### Go Style
- Use `gofmt` for consistent formatting
- Follow effective Go guidelines
- Use proper package naming
- Implement proper interfaces where needed
- Use proper error types and custom errors

### Documentation
- Add proper godoc comments for exported functions
- Include examples in documentation
- Document complex business logic
- Add README updates for new features

## Performance Considerations
- Check for proper resource limits and requests
- Ensure efficient use of Kubernetes API calls
- Validate proper caching strategies
- Check for memory leaks in long-running processes
- Ensure proper cleanup of resources

## Specific Project Rules

### CRD Management
- Validate all CRD schema definitions
- Ensure proper versioning strategy
- Check for backward compatibility
- Validate webhook configurations

### Volume Snapshot Logic
- Validate snapshot creation and deletion logic
- Check for proper error handling in snapshot operations
- Ensure proper cleanup of failed snapshots
- Validate snapshot group management

### Testing Requirements
- All new features must have unit tests
- Controller logic must have integration tests
- CRD changes must have validation tests
- Performance critical code must have benchmarks

## Review Checklist
- [ ] Security vulnerabilities addressed
- [ ] Proper error handling implemented
- [ ] Tests added for new functionality
- [ ] Documentation updated
- [ ] Code follows project conventions
- [ ] Performance impact considered
- [ ] Backward compatibility maintained
- [ ] Proper logging and monitoring added
