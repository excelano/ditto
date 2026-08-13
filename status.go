package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// statusLabelWidth is the width of the label column, set by the longest label
// ("converters") so every value starts in the same place.
const statusLabelWidth = 10

// cmdStatus reports where a project stands, and changes none of it.
//
// The facts already exist, scattered across the commands that compute them on
// the way to doing something: how many targets are out of date is what `build
// -n` decides, which files in src/ nobody declared is what `scan` reports, what
// sits in dist/ that no target produces and which converters this machine is
// missing are what `check` examines, and where a publish would land is in the
// manifest. Picking a project back up after a fortnight otherwise means running
// three commands and holding the answers in your head.
//
// The line against `check` is that status counts and check diagnoses. Status
// says two declared inputs are missing and names the command that will say
// which; it does not reprint check's findings in a second voice. That is why it
// converts nothing, writes nothing, touches no network, and exits 0 whatever it
// finds: the state it describes is the answer, not a verdict on the answer.
func cmdStatus(args []string) error {
	if len(args) != 0 {
		return unknownArg("status", args[0])
	}

	root, err := findProjectRoot()
	if err != nil {
		var absent *notAProjectError
		if errors.As(err, &absent) {
			// Not being in a project is a true answer to "where does this
			// stand", so it is reported rather than raised. A status command
			// that fails when the status is "there is no project here" has
			// answered a different question than the one asked. Every other
			// verb treats this as an error because every other verb was about
			// to do something that needs a project.
			fmt.Printf("No ditto project here: %v.\n", err)
			fmt.Println("Run 'ditto new <name>' to start one, or 'ditto init' to build one around the files already here.")
			return nil
		}
		// A project written against the pre-v0.2.0 manifest name is a broken
		// project rather than an absent one, and its error carries the rename
		// that fixes it, so it is raised like anywhere else.
		return err
	}
	m, err := loadManifestQuiet()
	if err != nil {
		// A manifest that will not parse leaves nothing to report. That is a
		// fault in the project, not a state of it.
		return err
	}

	statusLine("project", "%s  (%s)", projectLabel(m), shortenHome(root))
	statusLine("manifest", "%s, %s", manifestName, count(len(m.Targets), "target"))

	dist, distErr := m.resolveDist()
	if distErr != nil {
		dist = distDir // keep describing the rest against the default
	}

	reportBuild(m, dist, distErr)
	reportSources(m)
	reportDist(m, dist, distErr)
	reportPublish(m, dist)
	reportConverters(m, dist)
	return nil
}

// reportBuild counts what a build would reconvert, using the same freshness
// test build itself applies — including the recorded fingerprints, which build
// reads and which are cheap. The cache can only withhold freshness, so a lost
// one makes this overcount the work outstanding: wrong in the safe direction,
// and corrected by the build it just recommended.
//
// A target with a pipeline is never fresh, by the same rule build follows: the
// pipeline regenerates its inputs from outside ditto's view, so their
// timestamps cannot say whether the output is current. A project built entirely
// on pipelines therefore always reads as out of date, which is accurate.
func reportBuild(m *Manifest, dist string, distErr error) {
	if len(m.Targets) == 0 {
		statusLine("build", "nothing to build; run 'ditto scan --write' to declare what is in %s/", srcDir)
		return
	}
	if distErr != nil {
		statusLine("build", "cannot tell until dist is fixed, see below")
		return
	}
	fps := loadFingerprints()
	fresh, stale := 0, 0
	for _, t := range m.Targets {
		t.Reference = effectiveReference(m.Project, t)
		if isFresh(t, dist, fps) {
			fresh++
		} else {
			stale++
		}
	}
	switch {
	case stale == 0:
		statusLine("build", "%s up to date", count(fresh, "target"))
	case fresh == 0:
		statusLine("build", "%s out of date; run 'ditto build'", count(stale, "target"))
	default:
		statusLine("build", "%d up to date, %d out of date; run 'ditto build'", fresh, stale)
	}
}

// reportSources covers both directions between src/ and the manifest: files
// nobody declared, which is scan's question, and declared inputs that are not
// there, which is check's. Inputs belonging to a target with a pipeline are not
// counted as missing, since the pipeline exists to produce them.
func reportSources(m *Manifest) {
	covered := map[string]bool{}
	for _, t := range m.Targets {
		for _, in := range t.resolvedInputs() {
			covered[filepath.ToSlash(in)] = true
		}
	}
	uncovered, err := uncoveredInputs(covered)
	if err != nil {
		statusLine("sources", "no %s/ directory; run 'ditto check'", srcDir)
		return
	}

	missing := 0
	for _, t := range m.Targets {
		if len(t.Pipeline) > 0 {
			continue
		}
		for _, in := range t.resolvedInputs() {
			if _, err := os.Stat(filepath.Join(srcDir, in)); err != nil {
				missing++
			}
		}
	}

	if len(uncovered) == 0 {
		statusLine("sources", "every file in %s/ is covered", srcDir)
	} else {
		statusLine("sources", "%s in %s/ that no target covers; run 'ditto scan'", count(len(uncovered), "file"), srcDir)
	}
	if missing > 0 {
		statusCont("%s declared but not found; run 'ditto check'", count(missing, "input"))
	}
}

// reportDist describes what is on disk under dist/, and whether any of it is
// residue: a file no target produces is what a renamed or deleted target leaves
// behind, and publish without --delete ships it beside the real deliverables.
func reportDist(m *Manifest, dist string, distErr error) {
	if distErr != nil {
		statusLine("dist", "misconfigured: %v; run 'ditto check'", distErr)
		return
	}
	if _, err := os.Stat(dist); err != nil {
		statusLine("dist", "%s/ not built yet; run 'ditto build'", dist)
		return
	}
	files, err := countFiles(dist)
	if err != nil {
		statusLine("dist", "%s/ present, but unreadable: %v", dist, err)
		return
	}
	if files == 0 {
		// `ditto new` creates dist/, so an empty one is the ordinary state of a
		// project that has not been built rather than a missing directory.
		statusLine("dist", "%s/ is empty; run 'ditto build'", dist)
		return
	}
	orphans, err := orphanOutputs(m.Targets, dist)
	if err != nil || len(orphans) == 0 {
		statusLine("dist", "%s, all declared", count(files, "file"))
		return
	}
	statusLine("dist", "%s, %d produced by no target; run 'ditto clean'", count(files, "file"), len(orphans))
}

// reportPublish names where publish would send the deliverables. It says
// nothing about what is already there: knowing that costs a network round trip,
// and staying offline is most of what makes status worth typing.
func reportPublish(m *Manifest, dist string) {
	if m.Publish == nil || strings.TrimSpace(m.Publish.Root) == "" {
		statusLine("publish", "none; %s/ is itself the deliverable", dist)
		return
	}
	statusLine("publish", "%s", m.Publish.Root)
}

// reportConverters answers whether this machine can actually build this
// project, which is the question that separates "my project is wrong" from "my
// laptop is new". ditto orchestrates tools it does not ship, so a missing one
// is the expected first experience on a fresh machine rather than an exotic
// failure.
func reportConverters(m *Manifest, dist string) {
	needed := requiredConverters(m, dist)
	if len(needed) == 0 {
		statusLine("converters", "none needed yet")
		return
	}
	var present, missing []string
	for _, name := range sortedKeys(needed) {
		if _, err := exec.LookPath(needed[name]); err != nil {
			missing = append(missing, name)
		} else {
			present = append(present, name)
		}
	}
	if len(missing) == 0 {
		statusLine("converters", "%s: all present", strings.Join(present, ", "))
		return
	}
	if len(present) > 0 {
		statusLine("converters", "%s: present", strings.Join(present, ", "))
	}
	for i, name := range missing {
		text := fmt.Sprintf("%s: not on PATH%s", name, installHint(needed[name]))
		if i == 0 && len(present) == 0 {
			statusLine("converters", "%s", text)
			continue
		}
		statusCont("%s", text)
	}
}

// requiredConverters maps each converter this project needs, named as the
// manifest names it, to the command that has to be on PATH for it. A target
// whose extension pair has no built-in contributes nothing: that an unbuildable
// target exists is check's finding, and repeating it here would be the second
// voice status is meant to avoid.
func requiredConverters(m *Manifest, dist string) map[string]string {
	needed := map[string]string{}
	for _, t := range m.Targets {
		t.Reference = effectiveReference(m.Project, t)
		ins := srcInputs(t)
		if len(ins) == 0 || t.Output == "" {
			continue
		}
		cmd, err := converterCmd(t, ins, filepath.Join(dist, t.Output))
		if err != nil {
			continue
		}
		// A custom converter is displayed as written in the manifest rather
		// than as the absolute path resolveScript turned it into.
		name := cmd.Args[0]
		if t.Converter != "" {
			name = t.Converter
		}
		needed[name] = cmd.Args[0]
	}
	return needed
}

// projectLabel names the project, falling back to something readable rather
// than an empty column when [project] has no name. check reports the omission;
// status only has to render it.
func projectLabel(m *Manifest) string {
	if name := strings.TrimSpace(m.Project.Name); name != "" {
		return name
	}
	return "(unnamed)"
}

func statusLine(label, format string, a ...any) {
	fmt.Printf("%-*s  %s\n", statusLabelWidth, label, fmt.Sprintf(format, a...))
}

// statusCont adds a second line under the label already printed, for the facts
// that do not fit one line without becoming a paragraph.
func statusCont(format string, a ...any) {
	fmt.Printf("%-*s  %s\n", statusLabelWidth, "", fmt.Sprintf(format, a...))
}

// count renders a countable noun with its number, so the report says "1 file"
// and "3 files" rather than "1 file(s)".
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// shortenHome renders a path under the home directory with a leading ~, the
// inverse of expandHome, so the project line stays short enough to take in at a
// glance.
func shortenHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(os.PathSeparator)) {
		return "~" + p[len(home):]
	}
	return p
}
