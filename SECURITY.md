# Security Policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately through GitHub Security Advisories at https://github.com/excelano/ditto/security/advisories/new. If you would rather not use GitHub, email david.anderson@excelano.com instead. I aim to respond within seven days.

Please do not open public issues for security problems.

## Supported versions

Security fixes ship in a new tagged release. Update with `sudo apt upgrade ditto` (if installed from the Excelano apt repo) or by re-running the install script. There are no maintained release branches; only the latest release is supported.

## What ditto does

ditto is a build orchestrator that runs locally. It reads a `Manifest.toml`, converts text sources under `src/` into Office and HTML files under `dist/`, and — only when you run `ditto publish` — mirrors `dist/` to the configured root. It keeps no configuration or state of its own, sends no telemetry, and makes no network connections itself.

**A manifest can execute code.** A `[[target]]` may name a `converter` script and a `pipeline` of scripts, and `ditto build` runs them. Treat a `Manifest.toml` and its scripts the way you would any build file (a Makefile, an npm `scripts` block): a manifest from an untrusted source can run arbitrary commands on your machine with your privileges. Only build projects you trust, and review the `converter` and `pipeline` entries of a manifest you did not write before running `ditto build`.

## What ditto calls

ditto reimplements nothing; it shells out to separate tools on your `$PATH`: `md2docx`/`md2pptx`/`csv2xlsx` (office-convert), `cleave` (HTML), and `xsync` (the SharePoint push). Those tools run with your privileges and are governed by their own security posture — in particular, `ditto publish` to a SharePoint library delegates all authentication and transport to `xsync` (from the [xfiles](https://github.com/excelano/xfiles) suite), which signs in as you and can only reach what your account already can. ditto passes it the local `dist/` directory and the destination URL and does nothing with your credentials.
