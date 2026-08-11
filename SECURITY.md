# Security Policy

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Report them privately via GitHub's
[private vulnerability reporting](https://github.com/Baubap/pulumi-mlflow/security/advisories/new) (the
**"Report a vulnerability"** button on the repository's Security tab). If you can't use that, email
`security@baubap.com` with the details.

Please include:

- A description of the issue and its impact.
- Steps to reproduce (a minimal Pulumi program and the affected provider version).
- Any relevant configuration (auth mode, MLflow server version) — **redact secrets**.

We'll acknowledge your report, keep you updated on remediation, and credit you in the release notes unless you
prefer to remain anonymous.

## Supported versions

While the provider is on the `0.x` line, security fixes are released on the latest minor version. Once `1.0.0`
ships, the supported window will be documented here.

## Scope

This policy covers the provider code in this repository and the SDKs it publishes (`@baubap/mlflow`,
`baubap-mlflow`, the Go module). Vulnerabilities in MLflow itself should be reported to the
[MLflow project](https://github.com/mlflow/mlflow/security).
