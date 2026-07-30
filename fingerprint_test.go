package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The point of hashing the whole struct rather than a list of fields is that a
// field added to the manifest later cannot be forgotten here. This walks Target
// and asserts that setting each field on its own moves the hash: a field that
// does not is one whose edits would never rebuild the target.
func TestFingerprintCoversEveryTargetField(t *testing.T) {
	base := targetFingerprint(Target{})
	rt := reflect.TypeOf(Target{})
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		one := Target{}
		v := reflect.ValueOf(&one).Elem().Field(i)
		switch field.Type.Kind() {
		case reflect.String:
			v.SetString("changed")
		case reflect.Slice:
			v.Set(reflect.ValueOf([]string{"changed"}))
		default:
			t.Fatalf("%s is a %s, which this test does not know how to vary; teach it before adding the field", field.Name, field.Type.Kind())
		}
		if targetFingerprint(one) == base {
			t.Errorf("changing %s leaves the fingerprint identical, so editing it would never rebuild the target", field.Name)
		}
	}
}

func TestFingerprintMatchesOnlyWhatWasRecorded(t *testing.T) {
	target := Target{Input: "report.md", Output: "Report.docx"}
	fps := fingerprints{}
	if fps.matches(target) {
		t.Error("a target that was never recorded must not match")
	}
	fps.record(target)
	if !fps.matches(target) {
		t.Error("a target must match the entry just recorded for it")
	}
	target.View = "slides"
	if fps.matches(target) {
		t.Error("editing the target must break the match")
	}
}

// The whole reason this state exists: an edit to one target must not disturb
// the others' fingerprints, which is what the old manifest-mtime check could
// not express.
func TestFingerprintIsPerTarget(t *testing.T) {
	a := Target{Input: "a.md", Output: "A.docx"}
	b := Target{Input: "b.md", Output: "B.docx"}
	fps := fingerprints{}
	fps.record(a)
	fps.record(b)

	a.Reference = "brand/house.docx"
	if fps.matches(a) {
		t.Error("the edited target must go stale")
	}
	if !fps.matches(b) {
		t.Error("editing one target must leave its neighbours fresh")
	}
}

func TestFingerprintsRoundTripThroughDisk(t *testing.T) {
	t.Chdir(t.TempDir())
	target := Target{Input: "report.md", Output: "sub/Report.docx"}

	fps := fingerprints{}
	fps.record(target)
	if err := fps.save(); err != nil {
		t.Fatal(err)
	}
	if !loadFingerprints().matches(target) {
		t.Error("a saved fingerprint must survive a reload")
	}
}

// The cache may only ever withhold freshness. Every way of losing or corrupting
// it has to land on "rebuild", never on "assume it is current".
func TestUnreadableFingerprintsRebuild(t *testing.T) {
	target := Target{Input: "report.md", Output: "Report.docx"}

	t.Run("absent", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if loadFingerprints().matches(target) {
			t.Error("no state file must mean rebuild")
		}
	})

	t.Run("corrupt", func(t *testing.T) {
		t.Chdir(t.TempDir())
		mustWrite(t, fingerprintPath(), "{ this is not json")
		if loadFingerprints().matches(target) {
			t.Error("an unparseable state file must mean rebuild, not an error or a stale skip")
		}
	})
}

// save must not leave the previous state destroyed if it cannot write the new
// one, so it goes through a temporary file and a rename rather than truncating.
func TestSaveLeavesNoTemporaryFilesBehind(t *testing.T) {
	t.Chdir(t.TempDir())
	fps := fingerprints{}
	fps.record(Target{Input: "report.md", Output: "Report.docx"})
	for i := 0; i < 3; i++ {
		if err := fps.save(); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != fingerprintFile {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("%s holds %v, want only %s", stateDir, names, fingerprintFile)
	}
}

// dist/ is what publish mirrors to a client's library, so the state must not be
// written inside it.
func TestStateIsNotWrittenIntoDist(t *testing.T) {
	if filepath.IsAbs(stateDir) || stateDir == distDir {
		t.Fatalf("stateDir %q must be a project-relative directory of its own", stateDir)
	}
	t.Chdir(t.TempDir())
	fps := fingerprints{}
	fps.record(Target{Input: "report.md", Output: "Report.docx"})
	if err := fps.save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(distDir); err == nil {
		t.Errorf("saving state created %s/, which publish would ship to the client", distDir)
	}
}
