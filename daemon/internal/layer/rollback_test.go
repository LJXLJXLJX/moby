package layer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moby/moby/v2/daemon/graphdriver"
)

// Embed the interface so an unexpected driver operation fails the test.
type rollbackDriver struct {
	graphdriver.Driver
	root      string
	createErr error
	removeErr error
	layers    map[string]bool
	removed   []string
}

func (d *rollbackDriver) String() string { return "rollback-test" }

func (d *rollbackDriver) CreateReadWrite(id, parent string, opts *graphdriver.CreateOpts) error {
	if !strings.HasSuffix(id, "-init") && d.createErr != nil {
		return d.createErr
	}
	d.layers[id] = true
	return nil
}

func (d *rollbackDriver) Get(id, label string) (string, error) { return d.root, nil }
func (d *rollbackDriver) Put(id string) error                  { return nil }
func (d *rollbackDriver) Remove(id string) error {
	d.removed = append(d.removed, id)
	if d.removeErr != nil {
		return d.removeErr
	}
	delete(d.layers, id)
	return nil
}

func TestCreateRWLayerInitRollback(t *testing.T) {
	for _, failure := range []string{"create", "save", "cleanup", "success", "no-init"} {
		t.Run(failure, func(t *testing.T) {
			d := &rollbackDriver{root: t.TempDir(), layers: map[string]bool{}}
			store, err := newStoreFromGraphDriver(t.TempDir(), d)
			if err != nil {
				t.Fatal(err)
			}
			ls := store.(*layerStore)
			opts := &CreateRWLayerOpts{InitFunc: func(string) error { return nil }}
			original := errors.New("injected RW creation failure")
			switch failure {
			case "create", "cleanup", "no-init":
				d.createErr = original
				if failure == "cleanup" {
					d.removeErr = errors.New("injected remove failure")
				}
				if failure == "no-init" {
					opts.InitFunc = nil
				}
			case "save":
				// A regular file blocks the metadata mounts directory.
				if err := os.WriteFile(filepath.Join(ls.store.root, "mounts"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err = ls.CreateRWLayer("rollback", "", opts)
			if failure == "success" {
				if err != nil || len(d.layers) != 2 || len(d.removed) != 0 || len(ls.mounts) != 1 {
					t.Fatalf("successful creation changed: err=%v layers=%v removed=%v mounts=%v", err, d.layers, d.removed, ls.mounts)
				}
				return
			}
			if err == nil {
				t.Fatal("expected creation error")
			}
			if failure != "save" && !errors.Is(err, original) {
				t.Fatalf("lost original error: %v", err)
			}
			if len(ls.mounts) != 0 {
				t.Fatal("failed creation registered a mount")
			}
			if failure == "cleanup" {
				if len(d.removed) != 1 || !strings.HasSuffix(d.removed[0], "-init") {
					t.Fatalf("missing cleanup attempt: %v", d.removed)
				}
			} else if len(d.layers) != 0 {
				t.Fatalf("orphan graphdriver layers: %v", d.layers)
			}
		})
	}
}
