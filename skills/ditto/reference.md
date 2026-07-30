# ditto — reference

Complete contracts for authoring a `Ditto.toml`, writing a custom converter,
and understanding what `build`, `scan`, and `publish` do. The binary
(`ditto --help` for the command list, `ditto <command> --help` for one
command's flags) and the [README](https://github.com/excelano/ditto/blob/main/README.md)
are authoritative; this expands on them.

## Layout and conventions

- `src/` — the curated master tree you author and version. **Fixed**; not
  overridable. Inputs are named relative to it.
- `dist/` — built deliverables. Gitignored by the scaffold. Overridable per
  project (see `dist` below). Outputs are named relative to it.
- `Ditto.toml` — in the project root, authoritative. Not listed, not built.
- Commands work from anywhere inside the project. `build`, `scan`, and
  `publish` walk up from the current directory to the nearest `Ditto.toml` the
  way cargo finds `Cargo.toml`, then run from that root — so every path they
  print or resolve is relative to the root, not to where you are standing. When
  the two differ, ditto names the root on stderr.

## Manifest schema

### `[project]`

| Key | Required | Meaning |
|---|---|---|
| `name` | yes | Project name. |
| `dist` | no | Override where `build` writes and `publish` reads. A `~` is expanded. A path that resolves inside `src/` is **rejected** (building there would write into the source tree). Use for an external staging folder outside the repo. |
| `default_reference_docx` | no | Fallback `reference` for every `.docx` target that does not set its own. |
| `default_reference_pptx` | no | Fallback `reference` for every `.pptx` target that does not set its own. |

### `[publish]`

Optional. Omit the whole section to make the project build-only.

| Key | Required | Meaning |
|---|---|---|
| `root` | yes (if section present) | Destination for `publish`. An `https://…` value is a SharePoint library (mirrored via `xsync`); anything else is a local filesystem path (a `~` is expanded, mirrored by recursive copy). |

### `[[target]]` — one per deliverable

| Key | Required | Meaning |
|---|---|---|
| `input` | one of `input`/`inputs` | Primary source, relative to `src/`. Passed to the converter as the positional `<input>`. |
| `inputs` | one of `input`/`inputs` | Array of sources, relative to `src/`. The **first** is the positional `<input>`; the whole list reaches a custom converter as the `INPUTS` env var. Use when one deliverable is assembled from several sources. |
| `output` | yes | Destination, relative to `dist/` (or the `dist` override). Its **extension selects the converter**. May include subfolders and spaces: `"Deliverable 1/Asset Inventory.xlsx"`. |
| `reference` | no | Style template. For `.docx`/`.pptx` a pandoc reference doc (passed as `REFERENCE_DOC`); for `.html` a brand CSS (passed to cleave as `--brand`). Overrides the `[project]` default for its type. |
| `view` | no | cleave view for `.html` output (passed as `--view`, and to a custom converter as `VIEW`). |
| `converter` | no | Path to a script that overrides the built-in for this target. |
| `pipeline` | no | Array of scripts run in order **before** the converter, each with no arguments; first non-zero exit fails the target. |

Two targets may not write the same `output` — a build with a duplicate output
fails rather than silently letting one clobber the other. Each source extension
`scan --write` recognizes maps to a default output: `.md`/`.markdown` → `.docx`,
`.csv` → `.xlsx`. Everything else it skips and leaves for you to declare.

## Converter dispatch

For each target ditto resolves a command from `(primary input extension, output
extension)`:

1. If `converter` is set, it wins outright — the built-ins are not consulted.
2. Else the built-in for the pair is used:

| Inputs | Output | Command |
|---|---|---|
| one `.md`/`.markdown` | `.docx` | `md2docx <in> <out>` (office-convert) |
| one `.md`/`.markdown` | `.pptx` | `md2pptx <in> <out>` (office-convert) |
| one `.csv` | `.xlsx` | `csv2xlsx <in> <out>` (office-convert) |
| several `.csv` | `.xlsx` | `csv2xlsx <in>… -o <out>` — one sheet per CSV, named by filename stem, in listed order |
| one `.md`/`.markdown`/`.csv`/`.tsv`/`.ics`/`.ical` | `.html` | `cleave <in> <out> [--view …] [--brand …]` |

3. Any other pair with no `converter` is an error naming the fix (`set converter
   = "…"`). Multiple `inputs` are only accepted by the multi-CSV `.xlsx`
   built-in; every other multi-input target needs a custom `converter`.

## Custom converter contract

A `converter` script is invoked as:

```
<converter> <input> <output>
```

- `<input>` is the primary source and `<output>` the destination, **both
  relative to the project root**. ditto creates the output's parent directories
  first. A non-zero exit fails that target (the build continues other targets and
  exits non-zero overall).
- Environment passed in addition to the inherited environment:
  - `INPUTS` — every source path, **absolute**, one per line (the full `inputs`
    list, or just `input`). A converter that changes its working directory should
    read from `INPUTS`, not the relative positional argument.
  - `REFERENCE_DOC` — the target's `reference`, if set.
  - `VIEW` — the target's `view`, if set.
- **Script resolution:** a path is resolved relative to the **project root**
  first (`build_calendar.py` and `converters/build_calendar.py` both mean a file
  in the project); a bare name not found there falls through to a `$PATH` lookup.
  The script is executed directly, so it must be executable (`chmod +x`) and
  carry a shebang (`#!/usr/bin/env python3`).

```toml
[[target]]
inputs = ["contracts_enriched.csv", "contracts_expired.csv"]
output = "Deliverable 3/Contract Action Calendar.xlsx"
converter = "converters/build_calendar.py"
```

## Pipeline (upstream steps)

`pipeline` lists scripts that **produce** a target's inputs — an extract, an
enrich — run in order before the converter, each with no arguments, first
non-zero exit failing the target. They run **before** the inputs are checked for
existence, because they are what create those inputs, so `ditto build` reproduces
the whole derivation rather than just the last hop. Same script resolution and
executable+shebang rules as a converter. A pipeline belongs to its target; two
targets that share a pipeline each run it, and **a target with a pipeline is
never skipped as up to date** — the pipeline refreshes inputs from outside the
project, so their timestamps say nothing about whether the output is current.

```toml
[[target]]
pipeline = ["extract_contracts.py", "enrich_contracts.py"]
input = "contracts_enriched.csv"
output = "Deliverable 3/Contract Action Calendar.xlsx"
converter = "converters/build_calendar.py"
```

## Prefix filters (`build` / `publish`)

Both take an optional prefix — the leading segment of an output path — to work on
one slice of a many-deliverable manifest:

- `ditto build "Deliverable 3"` builds only targets whose `output` is under
  `Deliverable 3/`.
- `ditto publish "Deliverable 3"` mirrors only `dist/Deliverable 3/` to the
  matching `Deliverable 3/` subfolder of the root.

The prefix matches on a **path boundary**, so `Deliverable 3` selects
`Deliverable 3/…` without catching `Deliverable 30/…`.

## Freshness (`build`)

Within whatever the prefix selects, a target is skipped when both checks pass.
Its output must be newer than **all** of: every resolved input under `src/`, the
effective `reference` (including a `[project]` default), and the `converter` and
`pipeline` scripts when those resolve to files in the project. And its own entry
in `Ditto.toml` must be unchanged since it was last built. Anything missing or
unreadable counts as stale, so a deleted input still reaches the build and
produces a real error rather than being silently skipped.

- `ditto build --force` (`-f`) reconverts everything selected.
- `ditto build --dry-run` (`-n`) prints the same `input -> output` lines a real
  build would and stops there: no converter runs, and no `pipeline` runs, so it
  is safe against a pipeline that writes into `src/`. Combine with `--force` to
  see the full set rather than only the stale ones.
- Editing `view`, `reference`, or `converter` on a target rebuilds that target.
  Editing a **different** target, or adding one, does not — the comparison is
  per entry, not against the manifest's timestamp.
- Converters resolved from `$PATH` (`md2docx`, `csv2xlsx`, `cleave`) are **not**
  stat'd — after upgrading pandoc or office-convert, run `--force` once.
- `ditto clean && ditto build` is the from-source guarantee.

The per-entry comparison is backed by `.ditto/fingerprints.json` at the project
root, written by `build` and removed by `clean`, gitignored by `new`/`init`. It
is not in `dist/` because `publish` mirrors `dist/` verbatim. Losing or deleting
it forces one full rebuild and nothing worse: it can withhold freshness but
never grant it, and the timestamp comparison above always has to pass on its
own. Do not hand-edit it, and do not commit it.

## `check`

Validates without converting, and exits non-zero if anything would fail:
missing inputs (skipped for `pipeline` targets, whose inputs do not exist yet),
duplicate outputs, a missing `reference` or `[project]` default reference,
`pipeline` and `converter` scripts that are absent or not executable, an
(input ext → output ext) pair with no built-in and no `converter`, and any
built-in converter command that is not on `$PATH`. It also emits notes, which
do not fail the check: an empty `[project] name`, an empty manifest, and files
in `dist/` that no target produces.

**Case collisions.** Outputs whose paths differ only in case — `D3/…` and
`d3/…`, `Report.docx` and `report.docx` — are distinct locally and a single
item on SharePoint, which is case-preserving but case-insensitive. Colliding
files overwrite each other on publish with nothing reported; colliding folders
merge into whichever spelling the library already holds, and `xsync` reports a
`nameAlreadyExists` failure for the other. Both file-level and folder-level
clashes are reported, at every depth. Because the hazard is the destination's,
this is an **error** when `[publish] root` is an `https://` URL and a **note**
otherwise. The usual source is mixing hand-named outputs with `scan --write`
ones, which mirror the `src/` tree's spelling.

## `clean`

Removes the resolved `dist/` tree and reports the file count; `-n` / `--dry-run`
reports without removing. It is the only verb that deletes local outputs —
`build` only writes, so a renamed `output` orphans its predecessor in `dist/`,
which `publish` would then ship. Because `dist` is overridable and `~`-expanded,
clean refuses a `dist` that resolves to the filesystem root, the home directory,
or any directory containing the project.

## `new` vs `init`

`new <name>` creates the directory and fails if it exists. `init` scaffolds the
current directory instead, for a folder of sources that predates the decision to
build it with ditto: it refuses if a `Ditto.toml` is already there, appends
`/dist/` to an existing `.gitignore` rather than replacing it, leaves an
occupied `src/` untouched (no `.gitkeep`), and takes the project name from the
directory name. If a parent directory is also a ditto project, it says so —
commands run in the new project will use the nearer manifest.

## `publish` behavior

- SharePoint root (`https://…`): shells out to `xsync <dist> <root>`, passing
  `--dry-run` (`-n`) and `--delete` through. `xsync` must be on `$PATH` (install
  [xfiles](https://github.com/excelano/xfiles)) or publish fails with that hint.
  Flags precede the positional arguments because `xsync` parses with Go's `flag`
  package. Delete confirmations prompt on a TTY. On a **dry run** ditto also
  echoes the exact `xsync` invocation it is about to make, shell-quoted, so you
  can paste it and add flags of your own — `--itemize-changes` most usefully.
- Local root: a recursive overwrite copy; `--delete` prunes destination files
  with no counterpart in the source; `-n` previews.
- Each target's `output` already encodes the destination subfolder and filename,
  so `dist/` **is** the publish layout — there is no separate staging remap.

**Expect on SharePoint:** the library rewrites Office files on upload (it binds
them to a content type), so the stored bytes differ from what was sent and the
remote size and hash never match your local copy again. The deliverable content
is intact. **A current xsync handles this** — change detection compares
modification times and consults size only once the timestamp has moved, so an
unchanged `.docx`/`.xlsx`/`.pptx` is skipped on the next publish.

If a publish keeps re-sending Office files that nobody edited, the xsync on
`$PATH` predates that fix: it compared size too, so the rewritten bytes read as
a change every run and cut a new document version each time. Confirm by checking
whether `xsync --help` lists `--itemize-changes`; if it doesn't, upgrade. When it
does, `xsync --dry-run --itemize-changes <dist> <root>` labels each line `new`,
`time`, `content`, or `forced`, which answers why any particular file was picked.

## Install

`./install.sh` builds the single Go binary into `~/bin/ditto` (version stamped
from `git describe` via ldflags). ditto has no runtime dependencies of its own;
the converters it calls (office-convert, cleave, xsync) are separate installs and
`install.sh` warns about any that are missing.
