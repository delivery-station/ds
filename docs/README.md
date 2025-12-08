# DS Documentation

Welcome to the DS (Delivery Station) documentation! This directory contains comprehensive guides for using and developing with DS.

## Table of Contents

### For Users

- **[Configuration Guide](configuration.md)** - Complete reference for configuring DS
  - Configuration file formats
  - Environment variables
  - CLI flags and precedence
  - Authentication setup
  - Cache configuration
  - All available options

### For Developers

- **[Architecture Documentation](architecture.md)** - System design and internals
  - High-level architecture
  - Core components
  - Plugin system design
  - Communication patterns
  - Design decisions

- **[Plugin Development Guide](plugin-development.md)** - Create your own plugins
  - Plugin contract and requirements
  - Development workflow
  - Using the DS client library
  - Testing and debugging
  - Distribution via OCI registries

- **[Plugin Signing Guide](PLUGIN_SIGNING.md)** - Secure plugin distribution
  - Generating key pairs
  - Signing plugin binaries
  - Signature verification
  - Trust management
  - Security best practices

- **[Signature Quick Reference](SIGNATURE_QUICK_REF.md)** - Quick commands for signing
  - Common signing workflows
  - Troubleshooting signatures

## Quick Links

- [Main README](../README.md) - Project overview and quick start
- [Contributing Guide](../CONTRIBUTING.md) - How to contribute
- [Code of Conduct](../CODE_OF_CONDUCT.md) - Community guidelines
- [Security Policy](../SECURITY.md) - Reporting vulnerabilities
- [Examples](../examples/) - Configuration examples and templates

## Getting Help

- **Questions**: Open a [discussion](https://github.com/delivery-station/ds/discussions)
- **Bug Reports**: Use the [bug report template](../.github/ISSUE_TEMPLATE/bug_report.yml)
- **Feature Requests**: Use the [feature request template](../.github/ISSUE_TEMPLATE/feature_request.yml)
- **Security Issues**: Follow the [security policy](../SECURITY.md)

## Documentation Structure

```
docs/
├── README.md                    # This file - documentation index
├── architecture.md              # System design and architecture
├── configuration.md             # Configuration reference
├── plugin-development.md        # Plugin development guide
├── PLUGIN_SIGNING.md           # Plugin signing and verification
└── SIGNATURE_QUICK_REF.md      # Quick reference for signing
```

## Contributing to Documentation

Documentation improvements are always welcome! When contributing:

1. Keep language clear and concise
2. Include code examples where helpful
3. Update the table of contents if adding new sections
4. Test any commands or code snippets
5. Follow the existing documentation style

See [CONTRIBUTING.md](../CONTRIBUTING.md) for more details.

## Building a Mental Model

If you're new to DS, we recommend reading in this order:

1. Start with the [main README](../README.md) to understand what DS does
2. Read the [Architecture Documentation](architecture.md) to understand how it works
3. Follow the [Configuration Guide](configuration.md) to set up DS for your needs
4. Explore the [examples](../examples/) to see real-world configurations
5. If building plugins, read the [Plugin Development Guide](plugin-development.md)

## Key Concepts

### Plugin-Based Architecture
DS is a **meta-application** - it doesn't directly handle artifacts. Instead, it manages plugins that do the actual work. Think of it like Terraform managing providers.

### Configuration Inheritance
All plugins receive DS configuration automatically through the Host Config gRPC service. DS resolves configuration precedence (flags, environment, files) on the host and supplies the effective values directly to plugins, eliminating ad-hoc environment parsing and keeping behavior consistent across the ecosystem.

### OCI-Native
Everything in DS is distributed as OCI artifacts:
- Plugins are OCI artifacts
- Bundles are OCI artifacts
- Manifests are OCI artifacts

This leverages existing registry infrastructure and tooling.

### Zero Trust
DS never auto-installs plugins from untrusted sources. You explicitly control what runs on your system through:
- Manual plugin installation
- Signature verification
- Trusted registry configuration

## Feedback

Found an issue in the documentation? Have a suggestion for improvement?

- Open an issue: [Documentation Issue](https://github.com/delivery-station/ds/issues/new?labels=documentation)
- Submit a PR with improvements
- Join the discussion: [Discussions](https://github.com/delivery-station/ds/discussions)

---

**Last Updated**: 2025-12-08
