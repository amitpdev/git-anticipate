# Contributing

Thank you for your interest in contributing to git-anticipate!

## Quick Start

```bash
git clone https://github.com/amitpdev/git-anticipate.git
cd git-anticipate
make build
make test
```

## Development

```bash
make build        # Build binary
make test         # Run tests
make fmt          # Format code
make lint         # Run linter
```

## Pull Requests

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run `make test` and `make fmt`
5. Commit with clear messages: `feat: add feature` / `fix: resolve bug`
6. Submit PR

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Add tests for new functionality
- Keep functions focused

## Testing

```bash
# Run all tests
make test

# Run specific test
go test -v -run TestDeletedFile
```

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
