# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Currently supported versions:

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of DS seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Please Do Not:

- Open a public GitHub issue for security vulnerabilities
- Disclose the vulnerability publicly before it has been addressed

### Please Do:

1. **Report via GitHub Security Advisories**: Go to the [Security tab](https://github.com/delivery-station/ds/security/advisories) of this repository and click "Report a vulnerability"

2. **Provide detailed information**:
   - Type of vulnerability
   - Full paths of source file(s) related to the manifestation of the vulnerability
   - Location of the affected source code (tag/branch/commit or direct URL)
   - Step-by-step instructions to reproduce the issue
   - Proof-of-concept or exploit code (if possible)
   - Impact of the vulnerability, including how an attacker might exploit it

3. **Wait for confirmation**: We will acknowledge your report within 48 hours and will send a more detailed response within 5 business days indicating the next steps in handling your report.

### What to Expect:

- We will investigate all legitimate reports and do our best to quickly fix the problem
- We will keep you informed about the progress towards resolving the vulnerability
- We will credit you in the security advisory (unless you prefer to remain anonymous)

## Security Best Practices

When using DS:

1. **Plugin Security**:
   - Only install plugins from trusted sources
   - Verify plugin signatures when available
   - Review plugin code before installation if possible

2. **Credential Management**:
   - Never commit credentials or tokens to version control
   - Use environment variables or secure credential stores
   - Regularly rotate credentials and tokens

3. **Configuration**:
   - Use HTTPS for all registry connections (avoid `insecure_registries` in production)
   - Set appropriate file permissions on config files containing credentials
   - Review and minimize plugin permissions

4. **Updates**:
   - Keep DS and plugins updated to the latest versions
   - Subscribe to security advisories for this project
   - Monitor dependencies for known vulnerabilities

## Security Features

DS includes several security features:

- **Plugin Signature Verification**: Optional cryptographic verification of plugin binaries
- **Isolated Execution**: Plugins run as separate processes with limited privileges
- **Credential Isolation**: Credentials are passed securely to plugins via environment variables
- **TLS/HTTPS**: All registry communications use secure protocols by default

## Disclosure Policy

When we receive a security bug report, we will:

1. Confirm the problem and determine affected versions
2. Audit code to find similar problems
3. Prepare fixes for all supported releases
4. Release security patches as soon as possible

We will coordinate disclosure timing with the reporter to ensure users have time to upgrade before public disclosure.

## Comments on this Policy

If you have suggestions on how this process could be improved, please submit a pull request or open an issue.
