---
name: ditto
description: >-
  Build a managed set of Office and HTML deliverables from text sources with the
  `ditto` CLI — cargo for documents. Use this when a project keeps its
  deliverables as Markdown/CSV under `src/` with a `Ditto.toml`, or when a
  task is to produce the Word/Excel/PowerPoint/HTML versions of an authored set
  and optionally publish them to a SharePoint library: `ditto build`, then
  `ditto publish`. ditto is the conductor, not a converter — it drives
  office-convert, cleave, and xsync and reimplements none of them. Do NOT
  hand-run `pandoc`, `openpyxl`, or a one-off python script to convert files a
  manifest already governs; do NOT reach for ditto to convert a single loose
  file (call `md2docx`/`csv2xlsx` from office-convert directly), to edit cell
  values (that is xled), or to query/aggregate a table (that is xql/DuckDB).
---

# ditto — cargo for Microsoft documents

`ditto` builds plain-text sources into the Office and HTML deliverables people
expect, the way `cargo` builds a crate. You author Markdown, CSV, and other text
under `src/`; ditto presses each file into its Word, Excel, PowerPoint, or
portable-HTML counterpart under `dist/`; and an optional publish step mirrors
`dist/` to a SharePoint library or a local folder. The text is the master you
version; the Office files are the impression you hand to people who live in Word
and Excel.

The authoritative sources for ditto's behavior are the binary itself
(`ditto --help`) and the [README](https://github.com/excelano/ditto/blob/main/README.md);
if anything here conflicts with them, they win. These recipes assume a ditto
that supports multiple `inputs` per target, `pipeline` pre-steps, a `[project]`
`dist` override, and `build`/`publish` prefix filters — verify with
`ditto --help` and rebuild from source (`./install.sh`) if a feature is missing.

## The one rule that decides whether ditto is the right tool

ditto manages a **set** of deliverables described by a manifest. The moment the
task is a *managed set* — an engagement's deliverables, a report pack, anything
authored as text that must exist as Office files and stay in sync — ditto is the
right tool, and the manifest is how you drive it. The moment the task is a
*single loose conversion* — "turn this one `notes.md` into a Word doc" with no
project around it — skip ditto and call the underlying converter directly
(`md2docx notes.md notes.docx`). ditto's value is the manifest, the build set,
and the publish step; without a set there is nothing for it to conduct.

ditto **reimplements nothing**. It conducts tools that already exist and must be
on `$PATH`:

- [office-convert](https://github.com/anderix/nursery/tree/main/office-convert) — `md2docx`, `md2pptx`, `csv2xlsx` (pandoc + openpyxl).
- [cleave](https://github.com/anderix/axe) — portable self-contained HTML.
- [xsync](https://github.com/excelano/xfiles) — the SharePoint mirror, for `publish`.
  Want a build whose `--help` lists `--itemize-changes`; older ones re-upload
  every Office file on every publish (see the publish section of `reference.md`).

So the boundary is: **ditto orchestrates; it is not the converter and not the
SharePoint client.** Reach past ditto to a converter directly only for a genuine
one-off outside any project. For editing the *values* in a CSV before it builds,
that is [xled](https://github.com/excelano/xled); for querying or reshaping a
table, that is [xql](https://github.com/excelano/xql)/DuckDB — do that work
first, in `src/`, then let ditto build.

## The manifest is authoritative

`Ditto.toml` in the project root lists every deliverable. **If a file is not
listed, it is not built** — there is no implicit globbing. The verbs walk up to
the nearest `Ditto.toml` like cargo, so they run from any subdirectory and their
paths stay relative to the root. A minimal project:

```toml
[project]
name = "parsolvo-assessment"

[publish]                              # optional; omit to skip publishing
root = "https://parsolvo.sharepoint.com/sites/Engagement/Shared Documents/Deliverables"

[[target]]
input = "assessment.md"                # relative to src/
output = "Deliverable 1/Assessment.docx"   # relative to dist/
reference = "brand/excelano.docx"      # optional styling template

[[target]]
input = "inventory.csv"
output = "Deliverable 1/Asset Inventory.xlsx"
```

**The output extension picks the converter.** This is the rule an agent most
often gets wrong when reinventing the pieces: you do not tell ditto which engine
to run, you *name the output file*, and the extension chooses. The same
`assessment.md` builds a Word document in one target and a slide deck in another
just by ending one output `.docx` and the other `.pptx`.

| Input | Output | Engine |
|---|---|---|
| `.md` | `.docx` | office-convert (pandoc) |
| `.md` | `.pptx` | office-convert (pandoc) |
| `.csv` | `.xlsx` | office-convert (openpyxl) |
| several `.csv` | `.xlsx` | office-convert — one sheet per CSV |
| `.md` / `.csv` / `.ics` | `.html` | cleave |

Anything outside that table needs a `converter` on the target (see
`reference.md`). `reference` is a style template (a pandoc reference doc for
`.docx`/`.pptx`, a brand CSS for `.html`); `view` is a cleave view for HTML.

## The verbs

| Verb | Does | Reach for it when |
|---|---|---|
| `ditto new <name>` | Scaffold `src/`, `dist/`, `Ditto.toml`, `.gitignore` | Starting a new deliverable project |
| `ditto init` | Same scaffold, in a directory that already holds sources | Adopting an existing folder of Markdown/CSV |
| `ditto check` | Validate the manifest — inputs, references, scripts, converters on PATH, outputs colliding case-insensitively — without building | After hand-editing the manifest; before a first publish; on a machine that may lack the converters |
| `ditto build [prefix] [--force] [-n]` | Build every out-of-date target (or only those whose output is under `prefix/`) into `dist/` | Producing the Office/HTML files; iterating on one deliverable with a prefix |
| `ditto scan [--write]` | Report `src/` files no target covers; `--write` appends a target per uncovered file with default output | After adding sources, to catch anything the manifest misses |
| `ditto clean [-n]` | Remove `dist/` | Clearing outputs orphaned by a renamed target, before a publish |
| `ditto publish [prefix] [-n] [--delete]` | Mirror `dist/` to the `[publish] root` (SharePoint via xsync, or a local copy) | Handing the built set to the client library |

`build` **skips targets that are already up to date** — output newer than its
inputs, reference, converter script, and the manifest. `--force` reconverts
everything. Two exceptions: a target with a `pipeline` always rebuilds (the
pipeline regenerates inputs from outside ditto's view), and converters on
`$PATH` are not tracked, so `--force` once after upgrading pandoc. For a
guaranteed-from-source set, `ditto clean && ditto build`. `-n` / `--dry-run`
lists the targets that decision selects and runs neither converter nor
pipeline, which is the safe way to see the blast radius of a `--force`.

Every command takes `-h` / `--help`; `ditto <command> --help` is authoritative
over this table.

`clean` is the only thing that removes outputs. `build` only writes, so a
renamed `output` leaves the old file in `dist/`, and `publish` without
`--delete` would ship both — `check` names those orphans.

`scan` with no flag only reports — the bare command tells you the state, it never
mutates. `--write` appends (it does not rewrite the manifest), so hand-edited
names, ordering, and comments survive. It only fills the formats it can guess
(`.md`→`.docx`, `.csv`→`.xlsx`); a deck, HTML, or an unknown extension it leaves
for you to declare.

## The typical workflow

```sh
ditto new client-assessment          # scaffold the project (or: cd into an
cd client-assessment                 #   existing folder and run 'ditto init')
# ... author sources under src/ (assessment.md, inventory.csv, ...) ...
ditto scan --write                   # add a target per source, default outputs
# ... hand-edit Ditto.toml: fix output names, add references, add a .pptx target ...
ditto check                          # catch bad paths before converting anything
ditto build                          # press everything into dist/
ditto publish -n                     # preview the SharePoint mirror
ditto publish                        # push dist/ to the library
```

## Worked recipes

```sh
# Build only one deliverable while iterating (prefix = leading output segment).
# Matches on a path boundary, so "Deliverable 3" won't also catch "Deliverable 30".
ditto build "Deliverable 3"

# Same markdown, two deliverables — the output extension decides the engine.
#   [[target]] input="brief.md"  output="Executive Brief.docx"
#   [[target]] input="brief.md"  output="Board Deck.pptx"
ditto build

# Apply one house style to every .docx/.pptx without repeating `reference`:
#   [project]
#   default_reference_docx = "brand/excelano.docx"
#   default_reference_pptx = "brand/excelano.pptx"
# A target's own `reference` still wins.

# Several CSVs into one multi-tab workbook, no custom code (built-in):
#   [[target]] inputs=["active.csv","expired.csv"]  output="Asset Register.xlsx"
ditto build

# Preview a publish, then mirror only one deliverable's subfolder:
ditto publish -n "Deliverable 3"
ditto publish "Deliverable 3"

# Prune remote files that dist/ no longer has (destructive — confirm on a TTY):
ditto publish --delete
```

## When to stop and reach past ditto

- **A single loose file, no project** → call the converter directly:
  `md2docx notes.md notes.docx`, `csv2xlsx data.csv data.xlsx`. ditto is for a
  managed set, not a one-shot conversion.
- **Fix or clean cell values before building** (strip currency, pad leading
  zeros, normalize case) → [xled](https://github.com/excelano/xled) on the CSV in
  `src/` first, then `ditto build`.
- **Query, join, aggregate, or reshape a table** → xql/DuckDB, producing a clean
  CSV into `src/`; ditto builds it.
- **Custom presentation logic** (a colored, formula-bearing workbook; a
  templated deck) → set `converter` on the target and let ditto call your script
  with the `INPUTS`/`REFERENCE_DOC`/`VIEW` contract. ditto still conducts; your
  script is just another engine.

See `reference.md` in this directory for the complete manifest schema (every
field), the custom-converter and `pipeline` contracts, prefix semantics, and the
SharePoint upload behavior to expect on publish.
