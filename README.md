# ditto

ditto builds plain-text sources into the Microsoft Office and HTML deliverables
people expect, the way `cargo` builds a crate. You author Markdown, CSV, and
other text under `src/`; ditto presses each file into its Word, Excel,
PowerPoint, or portable-HTML counterpart under `dist/`; and an optional publish
step mirrors `dist/` to a SharePoint library or a local folder. The text is the
master you version; the Office files are the impression you hand to people who
live in Word and Excel.

The name is the ditto machine, the old spirit duplicator: the source and the
built document are two pulls of the same master, not a copy.

ditto reimplements nothing. It conducts tools that already exist —
[this2that](https://github.com/excelano/this2that)
for Markdown and CSV conversion, [cleave](https://github.com/excelano/axe) for
portable HTML, and [xsync](https://github.com/excelano/xfiles) for the
SharePoint push. Install whichever of those you need; ditto warns about what is
missing.

## The manifest is authoritative

`Ditto.toml` lists every deliverable. If a file is not listed, it is not
built — there is no implicit globbing. Each target names an input under `src/`,
an output under `dist/`, and optionally a style template, a view, or a custom
converter.

The manifest also marks the project root. Every command that works on a project
walks up from wherever you are to the nearest `Ditto.toml`, so it works from
inside `src/` or any other subdirectory rather than only at the top. Paths in
the output stay relative to the root; when that is not where you are standing,
ditto says which directory it settled on.

```toml
[project]
name = "contoso-assessment"

[publish]                              # optional; omit to skip publishing
root = "https://contoso.sharepoint.com/sites/Engagement/Shared Documents/Deliverables"

[[target]]
input = "assessment.md"                # relative to src/
output = "deliverable1/drafts/Assessment.docx"   # relative to dist/
reference = "brand/excelano.docx"      # optional styling template

[[target]]
input = "inventory.csv"
output = "deliverable1/supporting/Asset Inventory.xlsx"
```

The output extension picks the converter, so the same Markdown file can build a
Word document in one target and a slide deck in another just by naming
`.docx` in one and `.pptx` in the other. Markdown builds Word and PowerPoint,
CSV builds Excel (several CSVs can become one workbook, a sheet each), and
Markdown, CSV and calendar files build portable HTML. For anything else, a
target names its own `converter` script, and a `pipeline` of upstream scripts can
regenerate a target's inputs before it builds. The skill's
[reference](skills/ditto/reference.md) has the full manifest schema, the
dispatch table, and the converter and pipeline contracts.

## Workspaces

An engagement whose phases are authored separately and shipped together can be
a workspace: a root that owns no deliverables of its own and runs a verb across
its members.

```toml
[workspace]
members = ["phase-1", "phase-2", "closeout"]
```

Every project verb runs in each member, and naming one as the first argument
runs it there alone: `ditto build phase-2`. To filter by prefix inside a member,
stand in the member: `cd phase-2 && ditto build D3`. Each member keeps its own
`Ditto.toml`, `src/`, `dist/`, and `[publish] root`, and a member that fails does
not stop the others.

## Profiles

A deliverable set is worked one way and shipped another: for days against a
plain template, once against the branded one. A profile restyles a whole build
rather than one target.

```toml
[profile.draft]
reference_docx = "brand/plain.docx"
reference_pptx = "brand/plain.pptx"
```

`ditto build` uses no profile; `ditto build --profile draft` applies the
overrides, which win over both the `[project]` default and a target's own
`reference`. Draft and final share one `dist/`: switching profiles rebuilds what
the profile restyles and leaves everything else alone.

## Commands

```sh
ditto new <name>      # scaffold src/, dist/, Ditto.toml, .gitignore
ditto init            # scaffold a project around the files already here
ditto status          # report where the project stands, changing nothing
ditto check           # validate the manifest without building anything
ditto build           # build every target whose output is out of date
ditto build <prefix>  # build only targets whose output is under <prefix>/
ditto build --force   # rebuild every target, up to date or not
ditto build -n        # list what would be rebuilt, without converting anything
ditto build -p draft  # build with the named profile from the manifest
ditto scan            # report files in src/ that no target covers
ditto scan --write    # append a target per uncovered file, using defaults
ditto clean           # remove the built deliverables in dist/
ditto clean -n        # list what clean would remove, without removing it
ditto publish         # mirror dist/ to the publish root
ditto publish <prefix>  # mirror only dist/<prefix>/ to <root>/<prefix>/
ditto publish -n      # preview the publish without writing anything
ditto publish --delete  # also remove root files no longer in dist/
```

Every command explains itself: `ditto <command> --help` gives its arguments,
its flags, and what it is for.

`status` is the command to type when picking a project back up. It gathers
what the others each know a piece of onto one screen, converting nothing and
making no network call:

```
project     contoso-phase2  (~/clients/contoso/phase-2)
manifest    Ditto.toml, 14 targets
build       11 up to date, 3 out of date; run 'ditto build'
sources     2 files in src/ that no target covers; run 'ditto scan'
dist        14 files, 1 produced by no target; run 'ditto clean'
publish     https://contoso.sharepoint.com/Shared Documents/Phase 2
converters  csv2xlsx, md2docx, cleave: all present
```

`check` answers "will this build?" and "will this publish?" without converting
anything, including outputs that differ only in case, which SharePoint stores as
one item. `build` skips targets that are already up to date, the way cargo skips
crates that have not changed. `publish` mirrors `dist/` to a SharePoint library
through `xsync`, or to a local folder by plain copy; each target's `output`
already names the destination subfolder, so `dist/` is the publish layout. The
[reference](skills/ditto/reference.md) covers freshness, what SharePoint does to
Office files on upload, and the exit codes.

## Install

Every install line below ends with `ditto --install-skill`. That installs the [Claude Code skill](#use-it-from-claude-code) alongside the binary. Drop it if you do not use Claude Code — the CLI itself does not need it.

On Debian and Ubuntu, install from the Excelano apt repo so ditto updates with
the rest of your system. Add the repo once, then install:

```sh
curl -fsSL https://excelano.com/apt/setup.sh | sudo sh
sudo apt install ditto && ditto --install-skill
```

With a Go toolchain, install straight from the module:

```sh
go install github.com/excelano/ditto@latest && ditto --install-skill
```

That lands `ditto` in `$GOBIN` (usually `~/go/bin`). macOS ships its own
`/usr/bin/ditto`, so on that platform whichever directory comes first on your
`PATH` decides which one runs — put `$GOBIN` ahead of `/usr/bin` deliberately, or
build under a name of your choosing instead:

```sh
go build -o ~/bin/ditto .
```

ditto is a single Go binary with no runtime dependencies of its own; the
converters it calls are separate packages. `apt install ditto` recommends
both of them — [this2that](https://github.com/excelano/this2that),
which supplies `md2docx`, `md2pptx`, and `csv2xlsx` and pulls in pandoc, and
[cleave](https://github.com/excelano/axe), which bakes the portable HTML — so
every built-in output format works out of the box. `xsync` is suggested rather
than recommended, since it is needed only to publish; install it with
`apt install xfiles`. `ditto check` names whichever are missing.

## Use it from Claude Code

`skills/ditto/` is a [Claude Code](https://docs.claude.com/en/docs/claude-code)
skill so an AI coding agent drives ditto instead of hand-running the converters
it orchestrates. The binary installs it:

```sh
ditto --install-skill
```

That writes `~/.claude/skills/ditto/` and stamps in the skill's revision, which
`ditto -V` also prints, so a later run reports whether the skill has fallen
behind the binary rather than leaving you to notice. It is safe to re-run: an
unchanged skill reports `already current` and nothing is written.
`ditto --uninstall-skill` removes it.
Restart Claude Code afterwards, since skills are discovered at session start.

The skill is compiled into the binary, so this works the same however you
installed ditto — apt, `go install`, or a build from source.

The agent then reaches for `ditto build`/`ditto publish` on a manifest-governed
deliverable set, and hands a genuine one-off conversion straight to the
underlying converter.

## License

MIT. See [LICENSE](LICENSE).
