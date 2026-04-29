# Contributing to Canal

Thank you for your interest in contributing to Canal!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/yourusername/canal.git`
3. Create a branch: `git checkout -b feature/my-feature`
4. Make your changes
5. Test thoroughly
6. Submit a pull request

## Development Setup

```bash
cd canal
./scripts/setup.sh esp32s3
make TARGET=esp32s3
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Add comments to exported functions
- Keep functions focused and small
- Write descriptive commit messages

## Commit Messages

Format:
component: brief description
Longer explanation if needed.
Fixes #123

Examples:
kernel: add capability revocation support
domains/wifi: fix connection timeout handling
docs: update getting started guide

## Testing

Run tests before submitting:
```bash
make test
make test-integration
```

## Pull Request Process

1. Update documentation for any API changes
2. Add tests for new functionality
3. Ensure all tests pass
4. Update CHANGELOG.md
5. Request review from maintainers

## Areas for Contribution

- 🐛 Bug fixes
- 📝 Documentation improvements
- ✨ New domain implementations
- 🔧 Hardware platform support
- 🧪 Test coverage
- 📊 Performance optimization

## Questions?

- Open an issue for bugs
- Start a discussion for questions
- Join our community chat

## Code of Conduct

Be respectful, inclusive, and collaborative.
