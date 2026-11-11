# Security Policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately through GitHub Security Advisories at https://github.com/excelano/ditto/security/advisories/new. If you would rather not use GitHub, email david.anderson@excelano.com instead. I aim to respond within seven days.

Please do not open public issues for security problems.

## Supported versions

The latest release receives security fixes. Older versions are not supported. A fix ships in a new release; update the way you installed.

## What ditto can access

ditto is a build orchestrator that runs locally. It reads a `Ditto.toml` and the sources under `src/`, writes the built deliverables under `dist/` (or the directory the manifest's `dist` names), and, only when you run `ditto publish`, mirrors that directory to the configured root. It sends no telemetry and makes no network connections itself.

**A manifest can execute code.** A `[[target]]` may name a `converter` script and a `pipeline` of scripts, and `ditto build` runs them. Treat a `Ditto.toml` and its scripts the way you would any build file (a Makefile, an npm `scripts` block): a manifest from an untrusted source can run arbitrary commands on your machine with your privileges. Only build projects you trust, and review the `converter` and `pipeline` entries of a manifest you did not write before running `ditto build`.

ditto reimplements nothing; it shells out to separate tools on your `$PATH`: `md2docx`/`md2pptx`/`csv2xlsx` (this2that), `cleave` (HTML), and `xsync` (the SharePoint push). Those tools run with your privileges and are governed by their own security posture. In particular, `ditto publish` to a SharePoint library delegates all authentication and transport to `xsync` (from the [xfiles](https://github.com/excelano/xfiles) suite), which signs in as you and can only reach what your account already can. ditto passes it the local `dist/` directory and the destination URL and does nothing with your credentials.

## What ditto stores

ditto keeps no configuration or state outside your projects. Inside a project, `build` records a fingerprint of each target it builds in `.ditto/fingerprints.json` at the project root, which `clean` removes. `--install-skill` writes ditto's Claude Code skill to `~/.claude/skills/ditto/`, and `--uninstall-skill` removes that directory. There is no telemetry, no analytics, and no remote logging.

## Verifying releases

Every GitHub release includes a `checksums.txt` file listing SHA-256 hashes of all binary archives. Verify any download before running it:

    sha256sum -c checksums.txt --ignore-missing

Release artifacts are built by `build-release`, the release program in excelano/shipping, from a tagged commit using the goreleaser configuration in this repository (`.goreleaser.yml`). That build configuration is public and auditable.
