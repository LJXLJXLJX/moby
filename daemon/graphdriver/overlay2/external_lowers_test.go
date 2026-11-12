//go:build linux

package overlay2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestExternalStoreLowersRejectsInvalidPaths(t *testing.T) {
	d := &Driver{home: t.TempDir()}

	for _, tc := range []struct{ name, raw string }{
		{"relative path", "run/sandbox-erofs/l/w-1"},
		{"empty segment", ":"},
		{"trailing colon", externalLowerRoot + "/w-1:"},
		{"forbidden comma", externalLowerRoot + "/w,1"},
		{"forbidden newline", externalLowerRoot + "/w\n1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := d.storeExternalLowers("layer1", tc.raw)
			assert.ErrorContains(t, err, "overlay2.external_lowers")
		})
	}
}

func TestExternalLowersRoundTrip(t *testing.T) {
	home := t.TempDir()
	d := &Driver{home: home}

	// Layers without external lower metadata return nil.
	lowers, err := d.getExternalLowers("id1")
	assert.NilError(t, err)
	assert.Assert(t, lowers == nil, "expected nil, got %v", lowers)

	// Preserve the order of canonical paths stored one per line.
	want := []string{externalLowerRoot + "/t-abc", externalLowerRoot + "/w-123"}
	layerDir := filepath.Join(home, "id2")
	assert.NilError(t, os.MkdirAll(layerDir, 0o755))
	assert.NilError(t, os.WriteFile(
		filepath.Join(layerDir, externalLowerFile),
		[]byte(strings.Join(want, "\n")+"\n"), 0o644))

	got, err := d.getExternalLowers("id2")
	assert.NilError(t, err)
	assert.DeepEqual(t, want, got)
}

func TestExternalValidateLowerRejectsNonErofsAndEscapes(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root (writes below " + externalLowerRoot + ")")
	}
	d := &Driver{home: t.TempDir()}

	// Reject an ordinary directory below the allowed root.
	mp := filepath.Join(externalLowerRoot, "external-test-tmpfs")
	assert.NilError(t, os.MkdirAll(mp, 0o755))
	defer os.RemoveAll(mp)
	err := d.storeExternalLowers("layer1", mp)
	assert.ErrorContains(t, err, "not on an EROFS filesystem")

	// Reject a symlink that resolves outside the allowed root.
	link := filepath.Join(externalLowerRoot, "external-test-escape")
	assert.NilError(t, os.Symlink("/etc", link))
	defer os.Remove(link)
	err = d.storeExternalLowers("layer2", link)
	assert.ErrorContains(t, err, "outside allowed root")

	// Reject the allowed root itself.
	err = d.storeExternalLowers("layer3", externalLowerRoot)
	assert.ErrorContains(t, err, "outside allowed root")

	// Reject a missing path.
	err = d.storeExternalLowers("layer4", externalLowerRoot+"/no-such-mount")
	assert.ErrorContains(t, err, "overlay2.external_lowers")
}
