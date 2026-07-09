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

`Manifest.toml` lists every deliverable. If a file is not listed, it is not
built — there is no implicit globbing. Each target names an input under `src/`,
an output under `dist/`, and optionally a style template, a view, or a custom
converter.

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

| Input | Output | Engine |
|---|---|---|
| `.md` | `.docx` | office-convert (pandoc) |
| `.md` | `.pptx` | office-convert (pandoc) |
| `.csv` | `.xlsx` | office-convert (openpyxl) |
| `.md` / `.csv` / `.ics` | `.html` | cleave |

For anything the built-ins do not cover, set `converter` on the target to your
own script. It is called as `converter <input> <output>` with the output's
parent directories already created; a non-zero exit fails that target. Any
`reference` and `view` are passed to it as the `REFERENCE_DOC` and `VIEW`
environment variables.

When one deliverable is assembled from several sources — say a multi-tab
workbook built from three CSVs — list them all with `inputs` instead of `input`.
The first path is still passed as `<input>`, the positional argument, and the
whole set is handed to the converter as the `INPUTS` environment variable, one
absolute path per line. Naming every source here keeps the manifest honest about
what the deliverable depends on, and stops `ditto scan` from reporting the extra
files as uncovered. `inputs` is only meaningful with a custom `converter`; the
built-ins each take a single input.

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

Often the real source of a deliverable is further upstream than the file the
converter reads: a raw export gets pulled, then classified and enriched, and
only the result is ready to press into a workbook. List those upstream steps
with `pipeline` and ditto runs them in order before the converter, each with no
arguments, failing the target on the first non-zero exit. They run before the
inputs are checked for existence, because they are what produce those inputs, so
`ditto build` reproduces the whole derivation rather than just the last hop.

```toml
[[target]]
pipeline = ["extract_contracts.py", "enrich_contracts.py"]
input = "contracts_enriched.csv"
output = "deliverable3/Contract Action Calendar.xlsx"
converter = "converters/build_calendar.py"
```

## Commands

```sh
ditto new <name>      # scaffold src/, dist/, Manifest.toml, .gitignore
ditto build           # build every target into dist/
ditto scan            # report files in src/ that no target covers
ditto scan --write    # append a target per uncovered file, using defaults
ditto publish         # mirror dist/ to the publish root
ditto publish -n      # preview the publish without writing anything
ditto publish --delete  # also remove root files no longer in dist/
```

`publish` mirrors `dist/` to the `[publish] root`: a SharePoint library
(through `xsync`) or, when the root is a filesystem path, a plain recursive
copy. Each target's `output` already names the destination subfolder and
filename, so `dist/` is the publish layout — there is no separate staging step.

One thing to expect on SharePoint: the library rewrites Office files on upload
(it binds them to a content type), so the stored bytes differ from what you
sent. The deliverable content is untouched, but `xsync` will re-upload `.docx`,
`.xlsx`, and `.pptx` files on every publish rather than skipping them as
unchanged. This is SharePoint's behavior, not ditto's.

`ditto scan` with no flag only reports; `--write` appends a `[[target]]` for
each uncovered file using the default output (`.md` to `.docx`, `.csv` to
`.xlsx`), mirroring the `src/` tree. It appends rather than rewriting, so your
hand-edited names and comments are left alone. The formats it cannot guess —
`.md` to a deck, anything to HTML, unknown extensions — it leaves for you.

## Install

```sh
./install.sh
```

Builds the binary into `~/bin/ditto`. ditto is a single Go binary with no
runtime dependencies of its own; the converters it calls are separate installs.
