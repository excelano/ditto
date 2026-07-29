# ditto — reference

Complete contracts for authoring a `Manifest.toml`, writing a custom converter,
and understanding what `build`, `scan`, and `publish` do. The binary
(`ditto --help`) and the [README](https://github.com/excelano/ditto/blob/main/README.md)
are authoritative; this expands on them.

## Layout and conventions

- `src/` — the curated master tree you author and version. **Fixed**; not
  overridable. Inputs are named relative to it.
- `dist/` — built deliverables. Gitignored by the scaffold. Overridable per
  project (see `dist` below). Outputs are named relative to it.
- `Manifest.toml` — in the project root, authoritative. Not listed, not built.
- Commands run from the project root (the directory holding `Manifest.toml`).

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
targets that share a pipeline each run it (ditto rebuilds everything, tracking
nothing).

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
`Deliverable 3/…` without catching `Deliverable 30/…`. The prefix is the only
selection lever; there is no per-file freshness check.

## `publish` behavior

- SharePoint root (`https://…`): shells out to `xsync <dist> <root>`, passing
  `--dry-run` (`-n`) and `--delete` through. `xsync` must be on `$PATH` (install
  [xfiles](https://github.com/excelano/xfiles)) or publish fails with that hint.
  Flags precede the positional arguments because `xsync` parses with Go's `flag`
  package. Delete confirmations prompt on a TTY.
- Local root: a recursive overwrite copy; `--delete` prunes destination files
  with no counterpart in the source; `-n` previews.
- Each target's `output` already encodes the destination subfolder and filename,
  so `dist/` **is** the publish layout — there is no separate staging remap.

**Expect on SharePoint:** the library rewrites Office files on upload (it binds
them to a content type), so the stored bytes differ from what was sent and the
remote size and hash never match your local copy again. The deliverable content
is intact. **xsync 1.6.0 and later handle this** — change detection compares
modification times and consults size only once the timestamp has moved, so an
unchanged `.docx`/`.xlsx`/`.pptx` is skipped on the next publish. On xsync 1.5.x
it re-uploaded every Office file on every publish and cut a new document version
each time; if you still see that, check `xsync -V` before investigating further.
To see why any file was picked, run `xsync --dry-run --itemize-changes` against
the same source and destination — it labels each line `new`, `time`, `content`,
or `forced`.

## Install

`./install.sh` builds the single Go binary into `~/bin/ditto` (version stamped
from `git describe` via ldflags). ditto has no runtime dependencies of its own;
the converters it calls (office-convert, cleave, xsync) are separate installs and
`install.sh` warns about any that are missing.
