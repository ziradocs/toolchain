# Security Policy

SlideLang and DocLang convert plain-text markup into HTML/PDF/DOCX presentations and documents.
This document covers how to report a security vulnerability. For a description of the sanitizer
and the specific XSS/injection protections the renderer applies, see
[docs/architecture/sanitization.md](docs/architecture/sanitization.md).

## Supported versions

This project has not yet made a tagged `v1.0` release; it is pre-1.0 and moving fast. Until a
first stable release, security fixes are applied only to the `main` branch — there is no
long-term-support branch to backport to. Once tagged releases exist, this section will be updated
with a supported-versions table.

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report vulnerabilities privately using **[GitHub Private Vulnerability
Reporting](https://github.com/ziradocs/toolchain/security/advisories/new)** for this repository (the
"Security" tab → "Report a vulnerability"). This creates a private advisory visible only to you
and the maintainers, where we can discuss and coordinate a fix before any public disclosure.

There is no dedicated security email or domain yet — GitHub Private Vulnerability Reporting is
the only supported channel at this stage. This will be revisited if/when the project sets up a
dedicated security contact.

When reporting, please include:

- A description of the vulnerability and its impact.
- Steps to reproduce (a minimal `.slidelang`/`.doclang` input that triggers it is ideal).
- The affected component (parser, renderer, a specific CLI, the AI normalizer, etc.) and version
  or commit.
- A suggested fix, if you have one — not required.

### What to expect

This is a small, actively-developed open-source project without a dedicated security team, so
please treat the following as a good-faith goal rather than an SLA:

- **Acknowledgment:** we aim to respond to a new report within a few business days.
- **Triage:** we'll confirm whether we can reproduce it and give a rough sense of severity and
  timeline once we have.
- **Fix & disclosure:** once a fix is ready, we'll coordinate a disclosure timeline with you.
  Straightforward issues affecting `main` are generally fixed quickly; anything requiring a
  broader design change will take longer, and we'll keep you posted.

We credit reporters in the advisory/release notes unless you'd prefer to stay anonymous — let us
know your preference when you report.

## Scope

In scope: the `core`, `slidelang`, and `doclang` Go modules in this repository
— parsing, the AI normalizer, rendering (HTML/PDF/DOCX/Markdown), and the CLIs themselves,
including how they invoke Chromium/chromedp for diagrams, charts, maps, and PDF/offline rendering.

Generally out of scope: vulnerabilities in third-party dependencies (please report those upstream;
feel free to also flag them here if they materially affect this project and aren't yet tracked by
`govulncheck`/Dependabot), and issues that require an attacker to already have write access to the
input `.slidelang`/`.doclang` source files they're building (that's the trust boundary — this
project is a document/presentation compiler, not a sandbox for untrusted authors, unless stated
otherwise for a specific feature).
