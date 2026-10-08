//go:build linux

package overlay2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDsecStoreLowersRejectsInvalidPaths(t *testing.T) {
	d := &Driver{home: t.TempDir()}

	for _, tc := range []struct{ name, raw string }{
		{"relative path", "run/dsec/l/w-1"},
		{"empty segment", ":"},
		{"trailing colon", dsecLowerRoot + "/w-1:"},
		{"forbidden comma", dsecLowerRoot + "/w,1"},
		{"forbidden newline", dsecLowerRoot + "/w\n1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := d.dsecStoreLowers("layer1", tc.raw)
			assert.ErrorContains(t, err, "dsec.lowerdirs")
		})
	}
}

func TestDsecLowersRoundTrip(t *testing.T) {
	home := t.TempDir()
	d := &Driver{home: home}

	// Layers without external lower metadata return nil.
	lowers, err := d.getDsecLowers("id1")
	assert.NilError(t, err)
	assert.Assert(t, lowers == nil, "expected nil, got %v", lowers)

	// Preserve the order of canonical paths stored one per line.
	want := []string{dsecLowerRoot + "/t-abc", dsecLowerRoot + "/w-123"}
	layerDir := filepath.Join(home, "id2")
	assert.NilError(t, os.MkdirAll(layerDir, 0o755))
	assert.NilError(t, os.WriteFile(
		filepath.Join(layerDir, dsecLowerFile),
		[]byte(strings.Join(want, "\n")+"\n"), 0o644))

	got, err := d.getDsecLowers("id2")
	assert.NilError(t, err)
	assert.DeepEqual(t, want, got)
}

func TestDsecValidateLowerRejectsNonErofsAndEscapes(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root (writes below " + dsecLowerRoot + ")")
	}
	d := &Driver{home: t.TempDir()}

	// Reject an ordinary directory below the allowed root.
	mp := filepath.Join(dsecLowerRoot, "dsec-test-tmpfs")
	assert.NilError(t, os.MkdirAll(mp, 0o755))
	defer os.RemoveAll(mp)
	err := d.dsecStoreLowers("layer1", mp)
	assert.ErrorContains(t, err, "not on an EROFS filesystem")

	// Reject a symlink that resolves outside the allowed root.
	link := filepath.Join(dsecLowerRoot, "dsec-test-escape")
	assert.NilError(t, os.Symlink("/etc", link))
	defer os.Remove(link)
	err = d.dsecStoreLowers("layer2", link)
	assert.ErrorContains(t, err, "outside allowed root")

	// Reject the allowed root itself.
	err = d.dsecStoreLowers("layer3", dsecLowerRoot)
	assert.ErrorContains(t, err, "outside allowed root")

	// Reject a missing path.
	err = d.dsecStoreLowers("layer4", dsecLowerRoot+"/no-such-mount")
	assert.ErrorContains(t, err, "dsec.lowerdirs")
}
