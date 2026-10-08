//go:build linux

package overlay2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// DSec PoC unit tests（wip/03-phase-1.md §25）：
//   - 纯逻辑（无需 root）：dsec.lowerdirs 解析拒绝非法输入；持久化格式 round-trip
//   - root-gated：allowed root 下非 EROFS 目录拒绝、symlink escape 拒绝、root 本身拒绝
// 正向全链路（真实 EROFS + 挂载组合 + scratch）由 workspace 的
// scripts/smoke-p1a.sh / scripts/verify-p1a-risks.sh 覆盖（需要实验环境）。

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

	// 无文件 → nil（普通容器路径）
	lowers, err := d.getDsecLowers("id1")
	assert.NilError(t, err)
	assert.Assert(t, lowers == nil, "expected nil, got %v", lowers)

	// §17 持久化格式（一行一个 canonical path，尾随换行）→ 读出顺序一致
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

	// allowed root 下存在的普通目录（tmpfs，非 EROFS）→ 明确拒绝（guardrail L）
	mp := filepath.Join(dsecLowerRoot, "dsec-test-tmpfs")
	assert.NilError(t, os.MkdirAll(mp, 0o755))
	defer os.RemoveAll(mp)
	err := d.dsecStoreLowers("layer1", mp)
	assert.ErrorContains(t, err, "not on an EROFS filesystem")

	// allowed root 内的 symlink 指向 root 外 → 解析后拒绝
	link := filepath.Join(dsecLowerRoot, "dsec-test-escape")
	assert.NilError(t, os.Symlink("/etc", link))
	defer os.Remove(link)
	err = d.dsecStoreLowers("layer2", link)
	assert.ErrorContains(t, err, "outside allowed root")

	// allowed root 本身（非其下级 mountpoint）→ 拒绝
	err = d.dsecStoreLowers("layer3", dsecLowerRoot)
	assert.ErrorContains(t, err, "outside allowed root")

	// 不存在的路径 → 明确拒绝
	err = d.dsecStoreLowers("layer4", dsecLowerRoot+"/no-such-mount")
	assert.ErrorContains(t, err, "dsec.lowerdirs")
}
