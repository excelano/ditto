package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func cmdNew(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: ditto new <name>")
	}
	name := args[0]
	if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("%s already exists", name)
	}

	for _, d := range []string{filepath.Join(name, srcDir), filepath.Join(name, distDir)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	files := map[string]string{
		filepath.Join(name, manifestName):       starterManifest(name),
		filepath.Join(name, ".gitignore"):       "/dist/\n",
		filepath.Join(name, srcDir, ".gitkeep"): "",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}

	fmt.Printf("Created %s/\n", name)
	fmt.Printf("  %s/   your text sources (versioned)\n", srcDir)
	fmt.Printf("  %s/  built deliverables (gitignored)\n", distDir)
	fmt.Printf("  %s\n", manifestName)
	fmt.Printf("\nAdd files to %s/, then: ditto scan --write && ditto build\n", srcDir)
	return nil
}

func starterManifest(name string) string {
	return fmt.Sprintf(`[project]
name = %s

# Optional. Where 'ditto publish' mirrors dist/.
# A SharePoint library URL or a local path. Delete this section to skip publishing.
# [publish]
# root = "https://contoso.sharepoint.com/sites/Engagement/Shared Documents/Deliverables"

# Each target builds one file from src/ into dist/. The output extension picks
# the converter: .docx and .pptx (from .md), .xlsx (from .csv), .html (from
# .md/.csv/.ics via cleave). Run 'ditto scan --write' to fill these in from the
# files already in src/.
#
# [[target]]
# input = "report.md"                  # relative to src/
# output = "Final Report.docx"         # relative to dist/
# reference = "brand/excelano.docx"    # optional styling template
# view = "slides"                      # optional (cleave/html only)
# converter = "converters/custom.sh"   # optional override
`, tomlString(name))
}
