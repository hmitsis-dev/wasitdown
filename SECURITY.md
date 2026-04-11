# Security Policy

## Scope

wasitdown.dev is a read-only static site that aggregates public incident data. It makes no authenticated requests, stores no user data, and has no server-side runtime.

The main attack surface is the scraper itself (outbound HTTP to third-party status pages) and the PostgreSQL database.

## Reporting a Vulnerability

If you discover a security issue — such as a SQL injection in the scraper, a dependency with a known CVE, or anything else — please **do not open a public GitHub issue**.

Instead, report it privately via GitHub's [Security Advisories](../../security/advisories/new) feature, or email the maintainer directly (see the GitHub profile).

Please include:
- A description of the vulnerability
- Steps to reproduce or a proof of concept
- The potential impact

We'll respond as quickly as possible and coordinate a fix before any public disclosure.
