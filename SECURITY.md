# Security Policy

## Supported Versions

Foundry is under active development. Security fixes are applied to the latest supported release line and, when practical, the current `main` branch.

| Version | Supported |
| ------- | --------- |
| Latest release | Yes |
| Current `main` branch | Best effort |
| Older unreleased snapshots | No |

## Reporting a Vulnerability

Please do not report security vulnerabilities through public GitHub issues.

Instead, report them privately using one of the following channels:

- GitHub Security Advisories for this repository, if enabled
- The project security contact, if one is documented by the maintainers

If neither channel is available, open a minimal issue asking for a private contact method without disclosing the vulnerability details.

## What to include

When reporting a vulnerability, include as much of the following as possible:

- A clear description of the issue
- Affected version, commit, or branch
- Steps to reproduce
- Proof of concept, if available
- Impact assessment
- Any suggested remediation

## Response Process

The maintainers will make a best effort to:

1. Acknowledge receipt of the report
2. Reproduce and validate the issue
3. Assess severity and impact
4. Prepare and release a fix when appropriate
5. Coordinate disclosure timing as needed

Response times may vary depending on maintainer availability and issue complexity.

## Disclosure Policy

Please allow maintainers a reasonable amount of time to investigate and remediate a reported vulnerability before public disclosure.

Coordinated disclosure helps protect users and downstream adopters.

## Scope

Security issues may include, but are not limited to:

- Remote code execution
- Arbitrary file write or file read
- Authentication or authorization bypass in future admin functionality
- Path traversal
- Unsafe plugin or theme execution behavior
- Supply chain or dependency vulnerabilities with direct project impact

General bugs, feature requests, and non-security hardening suggestions should be filed through normal issue channels.