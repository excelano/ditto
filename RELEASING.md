# Releasing ditto

The release loop lives in `~/notes/releasing.md` — the ordered steps, the apt
step, the spent-tag rule, and the standing facts about tokens and secrets.
Failure recipes are in `~/notes/build_release_gotchas.md`. This file carries
what is true of ditto and not of its siblings.

| | |
|---|---|
| Loop | goreleaser |
| `apt-ship` argument | `ditto` |
| Homebrew | none, deliberately |
| winget | none — there is no Windows build |

**This is the shortest loop in the fleet.** Tag, let `release.yml` run,
`apt-ship ditto`. Steps 3, 5, and 6 of the shared procedure do not apply: the
`.deb` packages come out of the tagged job rather than a dispatched `deb.yml`,
there is no crate to publish, and there is no winget manifest to submit. Two
packages are expected, `ditto_<version>-?_amd64.deb` and its arm64 twin; apt is
the only channel a user installs from.

**Linux only, on purpose.** goreleaser builds `linux/amd64` and `linux/arm64`
and nothing else. macOS already ships `/usr/bin/ditto`, so a darwin binary or a
Homebrew formula under that name would collide with the system tool on exactly
the platform that would install it. macOS users build from source and pick their
own name; the reasoning is repeated at the top of `.goreleaser.yaml` so nobody
adds a darwin target without meeting it first. The absence of a Homebrew formula
is a decision, not an oversight, and `fleet -r` showing no formula for ditto is
the expected reading.

**`release.yml` has no `workflow_dispatch` input.** Where xql can recover a tag
that failed to trigger its workflow with `gh workflow run release.yml -f
tag=v1.2.3`, ditto cannot — the only route is deleting and re-pushing the tag,
which puts you straight into the spent-tag rule. Read it before doing that if
the tag has already produced a release. Adding the input is a small change and
worth making the next time this file is opened for another reason.

`install.sh` and `uninstall.sh` are attached to the release as extra files,
which is what lets a user pin an install to a release URL instead of the rolling
`main` branch. They ship as-is from the tagged commit, so a fix to either only
reaches pinned installs on the next release.

**ditto belongs to no metapackage.** `xtabular` and `xfiles` both pin their
members by name, and ditto is in neither, so a release never requires a
metapackage rebuild.

**The package `Recommends: office-convert, cleave`** — between them every
built-in converter ditto has — **and `Suggests: xfiles`**, which is needed only
to publish. If a converter is ever renamed or split, the `nfpms` block is what
has to change, and a stale name there fails quietly: apt resolves what it can
and a fresh install simply cannot build some formats.
