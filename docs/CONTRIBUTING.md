# Contributing to ZiraDocs CLI

> **See the repo-root [`CONTRIBUTING.md`](../CONTRIBUTING.md) first** — it's the authoritative
> guide for the DCO sign-off requirement, the multi-module dev/build/test setup, and code
> conventions. This page covers the softer "ways to contribute" and documentation-structure
> context; where the two disagree, the root file wins.

## Quick Start

1. **Set up your development environment** — see the root [`CLAUDE.md`](../CLAUDE.md#module-layout--the-gomod-gotcha) for the multi-module build/test commands.
2. **[Read the architecture overview](../CLAUDE.md)**
3. **Check the [issue tracker](https://github.com/ziradocs/toolchain/issues)** for current status and open work.
4. **Find an issue to work on or propose a new feature**

## Ways to Contribute

### Documentation
- Improve user guides and examples
- Fix typos and broken links
- Add missing documentation
- Translate documentation (future)

### Bug Reports
- Report issues with clear reproduction steps
- Include environment details and error messages
- Search existing issues first

### Feature Requests
- Propose new features with use cases
- Discuss implementation approaches
- Consider backward compatibility

### Code Contributions
- Fix bugs and implement features
- Improve performance and reliability
- Add tests and improve coverage
- Enhance developer experience

## Contribution Process

### 1. Before You Start
- Check existing issues and discussions
- Read relevant documentation
- Understand the project's goals and constraints
- Consider reaching out for guidance on large changes

### 2. Making Changes
1. **Fork** the repository
2. **Create a branch** for your changes (`git checkout -b feature/your-feature`)
3. **Make your changes** following our coding standards
4. **Add tests** for new functionality
5. **Update documentation** as needed
6. **Test thoroughly** - run the full test suite

### 3. Submitting Changes
1. **Commit** with clear, descriptive messages
2. **Push** to your fork
3. **Create a Pull Request** with:
   - Clear description of changes
   - Reference to related issues
   - Screenshots for UI changes
   - Test results

### 4. Review Process
- Maintainers will review your PR
- Address feedback and make requested changes
- Once approved, your changes will be merged

## Coding Standards

### Go Code Style
- Use `gofmt` for formatting
- Follow effective Go guidelines
- Use `golint` and `go vet`
- Organize imports properly

### Commit Messages
```
type(scope): brief description

Longer description if needed

Fixes #123
```

**Types:** `feat`, `fix`, `docs`, `test`, `refactor`, `style`, `chore`

### Branch Naming
- `feature/short-description`
- `fix/issue-number-description`  
- `docs/section-being-updated`

## Reporting Issues

### Bug Reports
Include:
- ZiraDocs version
- Operating system
- Go version (for build issues)
- Input files (minimal reproduction case)
- Expected vs actual behavior
- Complete error messages

### Feature Requests
Include:
- Use case and motivation
- Proposed behavior
- Examples of desired syntax/output
- Backward compatibility considerations

## Getting Help

### Questions & Discussion
- **GitHub Discussions:** General questions and ideas
- **Issues:** Specific bugs or feature requests
- **Discord:** Real-time chat (coming soon)

### Documentation
- **[User Docs](user/)** - Using ZiraDocs
- **[Developer Docs](developer/)** - Contributing and architecture
- **[Issue tracker](https://github.com/ziradocs/toolchain/issues)** - Status and roadmap

## Recognition

We appreciate all contributions! Contributors will be:
- Listed in the project README
- Mentioned in release notes for significant contributions
- Invited to join the core team for ongoing contributors

## Code of Conduct

This project follows the [Contributor Covenant](../CODE_OF_CONDUCT.md) — see the repo-root
`CODE_OF_CONDUCT.md` for the full text and how to report a concern.

## Contact

- **Security issues:** see [SECURITY.md](../SECURITY.md) — do not open a public issue.
- **Conduct concerns:** see [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md).
- **Everything else:** GitHub Discussions and Issues.
