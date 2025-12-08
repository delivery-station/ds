# Contributing to DS

Thank you for your interest in contributing to Delivery Station (DS)! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, constructive, and professional in all interactions. We aim to maintain a welcoming community for everyone.

## Getting Started

### Prerequisites

- Go 1.25 or later
- Make
- Git

### Setting Up Development Environment

1. **Clone the repository**:
   ```bash
   git clone https://github.com/delivery-station/ds.git
   cd ds
   ```

2. **Install dependencies**:
   ```bash
   make deps
   ```

3. **Build the project**:
   ```bash
   make build
   ```

4. **Run tests**:
   ```bash
   make test
   ```

5. **Run linter**:
   ```bash
   make lint
   ```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/my-feature
# or
git checkout -b fix/my-bugfix
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation updates
- `refactor/` - Code refactoring
- `test/` - Test additions or modifications

### 2. Make Changes

- Write clear, concise code
- Follow Go best practices and idioms
- Add tests for new functionality
- Update documentation as needed
- Keep commits focused and atomic

### 3. Test Your Changes

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run linter
make lint

# Format code
make fmt

# Build to ensure no compile errors
make build
```

### 4. Commit Your Changes

Write clear commit messages following this format:

```
<type>: <subject>

<body>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

Example:
```
feat: add support for TOML configuration

Add TOML configuration file support alongside existing YAML support.
Users can now use either config.yaml or config.toml.

Fixes #123
```

### 5. Push and Create Pull Request

```bash
git push origin feature/my-feature
```

Then create a pull request on GitHub with:
- Clear title describing the change
- Description of what changed and why
- Reference to related issues
- Screenshots (if UI changes)

## Code Style

### Go Code Style

Follow standard Go conventions:

```go
// Good: Clear function names, proper error handling
func LoadConfig(path string) (*Config, error) {
    if path == "" {
        return nil, fmt.Errorf("config path cannot be empty")
    }
    
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }
    
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }
    
    return &cfg, nil
}

// Good: Exported types have documentation
// Config represents the DS configuration.
type Config struct {
    // Registry is the default OCI registry.
    Registry string `yaml:"registry"`
    
    // CacheDir is the cache directory path.
    CacheDir string `yaml:"cache_dir"`
}
```

### Formatting

- Use `gofmt` to format all Go code
- Use tabs for indentation
- Keep lines under 100 characters when reasonable
- Add blank lines between logical sections

### Documentation

- Add godoc comments for all exported types, functions, and methods
- Use complete sentences with proper punctuation
- Include examples in documentation when helpful

```go
// Pull downloads an artifact from the specified registry.
// It returns an error if the artifact cannot be pulled.
//
// Example:
//   err := client.Pull(ctx, "ghcr.io/org/app:v1.0.0", os.Stdout)
func Pull(ctx context.Context, ref string, w io.Writer) error {
    // ...
}
```

## Testing

### Writing Tests

- Write unit tests for all new code
- Use table-driven tests when appropriate
- Mock external dependencies
- Test error cases

Example:

```go
func TestLoadConfig(t *testing.T) {
    tests := []struct {
        name    string
        path    string
        want    *Config
        wantErr bool
    }{
        {
            name: "valid config",
            path: "testdata/valid.yaml",
            want: &Config{Registry: "ghcr.io/delivery-station"},
            wantErr: false,
        },
        {
            name: "missing file",
            path: "testdata/missing.yaml",
            want: nil,
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := LoadConfig(tt.path)
            if (err != nil) != tt.wantErr {
                t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("LoadConfig() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Test Coverage

- Aim for >80% test coverage
- Focus on critical paths and error handling
- Check coverage with: `make test-coverage`

## Project Structure

```
ds/
├── cmd/ds/              # Main entry point
├── internal/            # Internal packages (not exported)
│   ├── cache/          # Cache implementation
│   ├── cmd/            # CLI commands
│   ├── communication/  # Event bus, state store
│   ├── config/         # Configuration management
│   ├── plugin/         # Plugin management
│   └── registry/       # OCI registry client
├── pkg/                # Public API (exported)
│   ├── client/         # DS client library
│   └── types/          # Shared types
├── docs/               # Documentation
├── examples/           # Example configurations and plugins
├── Makefile           # Build automation
└── go.mod             # Go module definition
```

### Package Guidelines

- **internal/**: Private implementation details, not exported
- **pkg/**: Public API for plugin developers
- **cmd/**: Command-line entry points

## Documentation

### Updating Documentation

When making changes, update relevant documentation:

1. **Code comments**: Update godoc comments
2. **README.md**: Update if user-facing changes
3. **docs/**: Update guides if architecture or features change
4. **Examples**: Update examples if APIs change

### Documentation Style

- Use Markdown for all documentation
- Keep line length reasonable (80-100 chars)
- Use code blocks with language hints
- Include examples and use cases
- Link to related documentation

## Pull Request Process

1. **Before submitting**:
   - Ensure all tests pass
   - Run linter and fix any issues
   - Update documentation
   - Rebase on latest main branch

2. **PR description should include**:
   - What changed
   - Why the change was made
   - How to test the change
   - Any breaking changes
   - Related issues

3. **Review process**:
   - Maintainers will review your PR
   - Address feedback and comments
   - Keep PR updated with main branch
   - Once approved, maintainer will merge

4. **After merge**:
   - Delete your branch
   - Celebrate! 🎉

## Bug Reports

### Before Creating a Bug Report

- Check existing issues to avoid duplicates
- Try to reproduce with latest version
- Gather relevant information

### Creating a Bug Report

Include:

1. **Description**: Clear description of the bug
2. **Steps to reproduce**:
   ```
   1. Run `ds plugin install foo`
   2. Run `ds foo bar`
   3. See error
   ```
3. **Expected behavior**: What should happen
4. **Actual behavior**: What actually happens
5. **Environment**:
   - OS and version
   - Go version
   - DS version
6. **Logs**: Relevant log output (use `--log-level debug`)
7. **Configuration**: Relevant config (redact sensitive data)

## Feature Requests

### Creating a Feature Request

Include:

1. **Problem**: What problem does this solve?
2. **Solution**: Proposed solution
3. **Alternatives**: Alternative solutions considered
4. **Examples**: Usage examples
5. **Impact**: Who benefits from this feature?

## Release Process

(For maintainers)

1. Update version in code
2. Update CHANGELOG.md
3. Create git tag: `git tag -a v1.0.0 -m "Release v1.0.0"`
4. Push tag: `git push origin v1.0.0`
5. CI will build and create GitHub release

## Questions?

- Open a discussion on GitHub
- Check existing documentation
- Ask in pull request or issue

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

## Recognition

Contributors will be recognized in:
- CONTRIBUTORS.md file
- Release notes
- GitHub contributor graph

Thank you for contributing to DS! 🚀
