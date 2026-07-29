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
[office-convert](https://github.com/anderix/nursery/tree/main/office-convert)
for Markdown and CSV conversion, [cleave](https://github.com/anderix/axe) for
portable HTML, and [xsync](https://github.com/excelano/xfiles) for the
SharePoint push. Install whichever of those you need; ditto warns about what is
missing.

## The manifest is authoritative

`Ditto.toml` lists every deliverable. If a file is not listed, it is not
built — there is no implicit globbing. Each target names an input under `src/`,
an output under `dist/`, and optionally a style template, a view, or a custom
converter.

The manifest also marks the project root. `build`, `scan`, and `publish` walk up
from wherever you are to the nearest `Ditto.toml`, so they work from inside
`src/` or any other subdirectory rather than only at the top. Paths in the
output stay relative to the root; when that is not where you are standing, ditto
says which directory it settled on.

```toml
[project]
name = "parsolvo-assessment"

[publish]                              # optional; omit to skip publishing
root = "https://parsolvo.sharepoint.com/sites/Engagement/Shared Documents/Deliverables"

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
`.docx` in one and `.pptx` in the other.

When a project has a house style, most `.docx` targets want the same reference
doc and most `.pptx` targets the same slide template. Set them once at the
`[project]` level with `default_reference_docx` and `default_reference_pptx`
rather than repeating `reference` on every target; a target's own `reference`
still overrides the default.

```toml
[project]
name = "parsolvo-assessment"
default_reference_docx = "brand/excelano.docx"
default_reference_pptx = "brand/excelano.pptx"
```

`src/` is fixed — it is the curated master tree, and `scan` reports anything in
it that no target covers, so it must hold only real sources. The output side is
not fixed: set `dist` at the `[project]` level to build and publish through a
directory other than the default `dist/` (a `~` is expanded), for instance an
external staging folder outside the repo. A `dist` that resolves inside `src/`
is rejected, since building there would write outputs into the source tree.

```toml
[project]
name = "parsolvo-assessment"
dist = "~/austin-contract2-stage"
```

| Input | Output | Engine |
|---|---|---|
| `.md` | `.docx` | office-convert (pandoc) |
| `.md` | `.pptx` | office-convert (pandoc) |
| `.csv` | `.xlsx` | office-convert (openpyxl) |
| several `.csv` | `.xlsx` | office-convert (one sheet per CSV) |
| `.md` / `.csv` / `.ics` | `.html` | cleave |

For anything the built-ins do not cover, set `converter` on the target to your
own script. It is called as `converter <input> <output>` with the output's
parent directories already created; a non-zero exit fails that target. Any
`reference` and `view` are passed to it as the `REFERENCE_DOC` and `VIEW`
environment variables.

The script path is resolved relative to the project root, so `build.py` and
`converters/build.py` both refer to a file in the project; a bare name not found
there falls through to a `$PATH` lookup. The script is run directly, so it must
be executable (`chmod +x`) and start with a shebang like `#!/usr/bin/env python3`.
The same holds for `pipeline` scripts below.

When one deliverable is assembled from several sources — say a multi-tab
workbook built from three CSVs — list them all with `inputs` instead of `input`.
The first path is still passed as `<input>`, the positional argument, and the
whole set is handed to the converter as the `INPUTS` environment variable, one
absolute path per line. The positional `<input>` and `<output>` are relative to
the project root, while `INPUTS` lines are absolute, so a converter that changes
its working directory should read sources from `INPUTS` rather than the
positional argument. Naming every source here keeps the manifest honest about
what the deliverable depends on, and stops `ditto scan` from reporting the extra
files as uncovered. `inputs` pairs with a custom `converter` for a deliverable
with real presentation logic; the one built-in that takes several inputs is
`.xlsx`, which turns several CSVs into a sheet each with no converter (below).

```toml
[[target]]
inputs = [
  "contracts_enriched.csv",
  "contracts_expired.csv",
  "contract_monthly_summary.csv",
]
output = "deliverable3/Contract Action Calendar.xlsx"
converter = "converters/build_calendar.py"
```

When several CSVs just need to land as separate tabs with no extra logic — each
already final, no coloring or derived columns — point `inputs` at them with an
`.xlsx` output and no `converter`. The built-in writes one sheet per CSV, named
after the input filename stem, in the order listed. Anything fancier than plain
tabs stays a custom `converter`.

```toml
[[target]]
inputs = ["active.csv", "expired.csv"]
output = "Asset Register.xlsx"
```

Often the real source of a deliverable is further upstream than the file the
converter reads: a raw export gets pulled, then classified and enriched, and
only the result is ready to press into a workbook. List those upstream steps
with `pipeline` and ditto runs them in order before the converter, each with no
arguments, failing the target on the first non-zero exit. They run before the
inputs are checked for existence, because they are what produce those inputs, so
`ditto build` reproduces the whole derivation rather than just the last hop. A
pipeline belongs to its target and runs whenever that target builds; two targets
that share the same pipeline each run it, in keeping with ditto rebuilding
everything rather than tracking what is already up to date.

```toml
[[target]]
pipeline = ["extract_contracts.py", "enrich_contracts.py"]
input = "contracts_enriched.csv"
output = "deliverable3/Contract Action Calendar.xlsx"
converter = "converters/build_calendar.py"
```

## Commands

```sh
ditto new <name>      # scaffold src/, dist/, Ditto.toml, .gitignore
ditto init            # scaffold a project around the files already here
ditto check           # validate the manifest without building anything
ditto build           # build every target whose output is out of date
ditto build <prefix>  # build only targets whose output is under <prefix>/
ditto build --force   # rebuild every target, up to date or not
ditto scan            # report files in src/ that no target covers
ditto scan --write    # append a target per uncovered file, using defaults
ditto clean           # remove the built deliverables in dist/
ditto clean -n        # list what clean would remove, without removing it
ditto publish         # mirror dist/ to the publish root
ditto publish <prefix>  # mirror only dist/<prefix>/ to <root>/<prefix>/
ditto publish -n      # preview the publish without writing anything
ditto publish --delete  # also remove root files no longer in dist/
```

`new` creates a project; `init` adopts one. Point `init` at a folder that
already holds your sources and it writes the manifest, adds `dist/` to an
existing `.gitignore` rather than replacing it, and leaves whatever is already
in `src/` alone. Follow either with `ditto scan --write`.

`check` answers "will this build?" without converting anything: every input
exists, no two targets write the same output, every reference and pipeline
script is where the manifest says, every output extension maps to a converter,
and every converter that implies — `md2docx`, `csv2xlsx`, `cleave` — is
installed on this machine. It also names files sitting in `dist/` that no
target produces. Run it after hand-editing the manifest, and on any machine
that has to build the project but may not have the same tools.

It answers "will this publish?" too. SharePoint and OneDrive are
case-preserving but case-insensitive, so outputs that differ only in case are
two paths locally and one item in the library: two such files overwrite each
other on upload with nothing reported, and two such folders merge. This is easy
to create without noticing, because `scan --write` names outputs after the
`src/` tree — a manifest with a hand-named `D3/…` target and a scanned `d3/…`
one asks for a folder the library cannot give you twice. `check` reports the
clash as an error when the manifest has an `https://` publish root, and as a
note otherwise.

On an engagement with many deliverables, a single manifest holds dozens of
targets but you are usually iterating on one. `build` and `publish` take an
optional prefix — the leading segment of an output path — to work on just that
slice: `ditto build D3` builds only the targets whose `output` is under `D3/`,
and `ditto publish D3` mirrors only `dist/D3/` to the matching `D3/` subfolder of
the root, so neither one pays the cost of touching every other deliverable. The
prefix matches on a path boundary, so `D3` selects `D3/…` without also catching
`D30/…`.

Within that selection, `build` skips targets that are already up to date, the
way cargo skips crates that have not changed: a target is rebuilt when its
output is older than any of its inputs, its styling reference, its converter
script, or the manifest itself — so changing a target's `view` or `reference`
rebuilds it just as editing the source does. `ditto build --force` reconverts
everything regardless.

Two things are deliberately outside that check. A target with a `pipeline`
always rebuilds, because the pipeline exists to regenerate inputs from
somewhere ditto cannot see and their timestamps prove nothing about whether the
result is current. And converters found on `$PATH` are not tracked, so after
upgrading pandoc or office-convert, run `--force` once to pick up the new
output. When you want the guarantee rather than the speed, `ditto clean &&
ditto build` reproduces the whole set from source.

`publish` mirrors `dist/` to the `[publish] root`: a SharePoint library
(through `xsync`) or, when the root is a filesystem path, a plain recursive
copy. Each target's `output` already names the destination subfolder and
filename, so `dist/` is the publish layout — there is no separate staging step.

One thing to expect on SharePoint: the library rewrites Office files on upload,
binding them to a content type, so the stored bytes differ from what you sent
and the remote size and hash never match your local copy again. The deliverable
content is untouched, and it costs you nothing: xsync compares modification
times and only looks at size once the timestamp has moved, so an unchanged
`.docx`, `.xlsx`, or `.pptx` is skipped on the next publish. If you do see every
Office file re-uploaded on every publish, cutting a new document version each
time, your xsync predates that change — check whether `xsync --help` lists
`--itemize-changes`, and upgrade if it doesn't. To find out why a particular
file is being sent, run `xsync --dry-run --itemize-changes` against the same
source and destination.

`ditto scan` with no flag only reports; `--write` appends a `[[target]]` for
each uncovered file using the default output (`.md` to `.docx`, `.csv` to
`.xlsx`), mirroring the `src/` tree. It appends rather than rewriting, so your
hand-edited names and comments are left alone. The formats it cannot guess —
`.md` to a deck, anything to HTML, unknown extensions — it leaves for you.

`ditto clean` removes `dist/`. Nothing else does: `build` only ever writes, so
renaming a target's `output` leaves the old file behind, and a later `publish`
without `--delete` ships both the current deliverable and the stale one to the
client library. `check` names those orphans and `clean` clears them. Because
`dist` is configurable and `~` is expanded, clean refuses to remove the
filesystem root, your home directory, or any directory containing the project.

## Install

On Debian and Ubuntu, install from the Excelano apt repo so ditto updates with
the rest of your system. Add the repo once, then install:

```sh
curl -fsSL https://excelano.com/apt/setup.sh | sudo sh
sudo apt install ditto
```

Anywhere else with a Linux binary, fetch the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/excelano/ditto/main/install.sh | sh
```

The script drops the binary into `/usr/local/bin` (or `~/.local/bin` if that is
not writable). Pin a version with `DITTO_VERSION=v0.1.0` or choose the directory
with `DITTO_INSTALL_DIR`. Uninstall with the matching
`uninstall.sh`, or `sudo apt remove ditto` if you installed via apt.

To build from source (and on macOS, where the system already ships its own
`/usr/bin/ditto`, so pick a different output name):

```sh
go build -o ~/bin/ditto .
```

ditto is a single Go binary with no runtime dependencies of its own; the
converters it calls (office-convert, cleave, xsync) are separate installs, and
`apt install ditto` recommends `pandoc` for the common Markdown-to-Word path.

## Claude Code skill

`skills/ditto/` is a [Claude Code](https://docs.claude.com/en/docs/claude-code)
skill so an AI coding agent drives ditto instead of hand-running the converters
it orchestrates. Install it into your skills directory:

```sh
cp -r skills/ditto ~/.claude/skills/
```

The agent then reaches for `ditto build`/`ditto publish` on a manifest-governed
deliverable set, and hands a genuine one-off conversion straight to the
underlying converter.
