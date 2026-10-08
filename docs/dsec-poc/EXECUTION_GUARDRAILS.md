# DSec PoC 执行 Guardrails（长期原则，imperative）

> 本文件面向所有在本 moby fork（`dsec-erofs-poc` 分支）上工作的 coding agent。
> 无论从哪个 session / subagent 进入，修改任何代码前必须先读本文件。
> workspace 侧对应入口：`/home/jianxiao/dsec-workspace/AGENTS.md` 的
> "DSec PoC Guardrails" 节；Phase 执行文档在 workspace `wip/`。

## A. 项目目标

不要重新实现 sandbox/runtime。本 PoC 只验证 DSec-like 路径：

```text
pre-mounted EROFS workspace/toolkit
        +
Docker image layers
        ↓
patched Moby rootfs composition
        ↓
OverlayFS
        ↓
Docker/runc container
```

最终经薄 Sandbox Adapter 接入 Harbor。

首要研究目标是提高大规模 sandbox 的批量启动效率和并发能力。Task image 解耦
主要用于缩短创建请求到 task ready 的时间，减少重复准备、拉取和物化；维护成本
是次要收益。后续性能设计见 workspace `docs/dsec-poc/PERFORMANCE.md`。

## B. Moby patch 必须保持最小化

Moby 只负责：消费 pre-mounted EROFS path，修改 rootfs lower composition。

禁止把以下能力塞进 Moby：

```text
artifact 下载 / S3 / artifact cache / EROFS image build / EROFS mount manager
refcount / GC / Harbor integration / sandbox HTTP API / scheduler / network orchestration
```

如果实现开始要求大量 Moby 新 abstraction，停下来重新检查设计。
"30 行 Go"不是机械行数限制，但必须视为重要 architecture signal。

## C. 不重新实现 Docker

禁止走向：自己管理 namespace/cgroup、自己生成 OCI bundle、自己维护
exec/kill/log/network、直接围绕 runc 做一套 runtime。

Docker/Moby 继续负责 container lifecycle、namespace、cgroup、network、
seccomp、capabilities、exec、logs、OCI image、cleanup。

只修改 rootfs composition。

## D. Phase Gate

严格按 Phase 推进，未过 Gate 不进下一阶段。特别地：

> **Phase 1A 完成后必须 STOP & REPORT，不允许自动继续 1B~1F。**
> 先汇报代码 diff 和实验结果，由人确认后再继续。

**当前进度补充（2026-10-08）**：Phase 1A～1D 已完成，历史 Gate、结果及 commit
继续有效；不得重跑、推翻、重构或重新编号。上述 1A 停止规则已履行，保留为历史
阶段纪律。当前停在 1D 后，待人工确认 1E；后续按 1E → 1F → 新增 1G 推进。
1F 为 A/B/C 并发对照，1G 为 task image 解耦与启动性能。Phase 2 / 3 保持原计划，
在新性能 Gate 完成及继续决策前暂缓。本次仅计划修订，不授权执行新实验。

## E. Phase 1A scope

Phase 1A 只做：

```text
1. 确认 classic overlay2 实际调用路径
2. 找到最终 lowerdir + mount(overlay) 位置
3. 找最小 per-container 参数 plumbing
4. 插入 pre-mounted EROFS lower
5. build patched dockerd
6. 启动独立实验 daemon
7. 单 container rootfs smoke test
8. 验证 lower 优先级
9. 验证 copy-up 到 Docker upper
10. 验证 EROFS 本身不变
```

Phase 1A 不做：

```text
Harbor / Sandbox Adapter / S3 / artifact service / refcount / GC
128 concurrency / 大规模 failure injection / production security hardening
containerd snapshotter migration / 复杂网络 / 未来扩展 abstraction
```

## F. 网络不是 Phase 1A 变量

实验 daemon 起步使用（daemon.json）：

```json
{ "bridge": "none", "iptables": false }
```

不要配置额外 Docker address pool。测试 image 提前 pull/load。
网络在真正需要 Harbor E2E 时再单独开启和验证。

## G. 实验 Docker 必须完全隔离

固定：

```text
DOCKER_DATA_ROOT=/data0/dsec-docker
```

不要使用 `/data0` 根本身。同时使用独立 data-root / exec-root / unix socket /
pidfile。不得污染系统 Docker（data-root=/data1/docker）。所有测试命令必须显式
指向实验 daemon（DOCKER_HOST=unix://$DSEC_DOCKER_SOCKET）。

## H. 当前工作区

项目根：`/home/jianxiao/dsec-workspace`；源码：`/home/jianxiao/dsec-workspace/moby`。
源码、文档、脚本、日志优先留在工作区；runtime-sensitive path 按当前机器实际
filesystem 决定（见 workspace `env.sh`）。

## I. source of truth

不得依据旧 design doc 猜当前 Moby 实现。每次关键修改前实际使用：

```bash
rg / git grep / git blame
```

确认真实调用链。如果当前 upstream Moby 与文档描述不同：以当前源码为准，
并把差异记录到 workspace `docs/dsec-poc/NOTES.md`。

## J. 不提前抽象

PoC 优先：最少代码、最短路径、明确 observable。不要为了"以后可能需要"提前引入
generic layer provider / plugin framework / storage backend interface /
artifact service interface / 复杂 configuration hierarchy。
只有出现真实第二个实现时才抽象。

## K. 每一步必须可观察

Moby 改动必须能通过日志或 inspect 明确看到：

```text
container ID / external EROFS lowers / Docker lower count
最终 lower 顺序 / UpperDir / WorkDir / MergedDir
```

不得只依赖"container 能启动"判断成功。最终必须在 host 上检查真实
OverlayFS mount options（findmnt）。

## L. 禁止静默 fallback

`dsec.lowerdirs` 无效 / EROFS path 不存在 / lower 插入失败 / OverlayFS mount
失败 → 必须明确失败。禁止忽略 DSec lower 退回普通 Docker container
（false positive）。

## M. Git hygiene

每个逻辑修改独立 commit。禁止把 Moby patch、实验脚本、大规模格式化、无关
refactor 混在同一个 commit。Phase 1A 结束必须报告：

```bash
git diff --stat
git log --oneline <baseline>..HEAD
```

## N. 保留失败结果

实验失败不是需要隐藏的结果。kernel/filesystem/Docker 异常记录到 workspace
`docs/dsec-poc/NOTES.md`；通过的 gate、命令、输出摘要记录到 workspace
`docs/dsec-poc/RESULTS.md`。不要反复修改测试条件直到"看起来成功"而不记录失败路径。

## O. 后续性能研究约束

- 区分 Burst Creation Throughput（单位时间创建并 ready 数）与 Steady-state
  Density（固定实际任务负载、SLO、资源限制下单节点稳定承载数）；分别报告。
- 比较 A 已缓存完整 OCI、B base + 每 sandbox tar.gz 解压/复制、C base + 共享
  pre-mounted EROFS；同 task 内容、权限、工具、依赖、环境、最终命令与 readiness。
  不预设 C 优于 A；A/C 验证对已缓存 OCI 的收益，B/C 验证免重复物化收益。
- 本地不包含完整 DSec 的 3FS 按需加载，不能外推为完整 DSec 性能收益。
- 保留已完成 correctness Gate，不跳过 1E。性能研究先小规模可复现，再扩数据规模
  与并发；至少一个真实 SWE/Harbor task，不能以空容器、sleep 或极小文件证明收益。
- 离线构建与 warm-start 分开，warm/cold cache 分开；无法控制缓存时标注条件。
  同机同限制、方案之间无资源竞争、多轮原始数据与失败样本、SHA/配置/命令均留存。
- 不为并发数字修改 Moby 调度、内存管理、CPU overcommit；现有 Moby 实现保持不变，
  后续数据证明必要时才讨论最小修改，不把 benchmark 或 artifact 管理塞入 Moby。
- 未经明确授权不得全局 drop_caches、Docker prune 或破坏性清理。资源限制、停止
  条件及 Gate 统一见 workspace `docs/dsec-poc/PERFORMANCE.md`，不另设冲突标准。

---

# Phase 1A 具体执行决定（随本 guardrails 一并固化）

## 1. Docker runtime 路径

- `DOCKER_DATA_ROOT=/data0/dsec-docker`（不用 /data0 根本身）
- exec-root / socket / pidfile 全部独立（见 workspace `env.sh`）
- 不修改、不复用系统 Docker 的 data-root

## 2. 网络

完全关闭 container 网络（`bridge:none` + `iptables:false`），不配地址池。

## 3. Moby 修改要求

核心：把已经 pre-mounted 的 EROFS path 在 OverlayFS mount 前动态插入 lowerdir，
并放在 Docker image layers 之上。动手前先在源码确认：

```bash
rg "CreateReadWrite" daemon
rg "StorageOpt" daemon
rg 'lowerdir=' daemon/graphdriver/overlay2
rg 'unix.Mount|mount\("overlay"' daemon/graphdriver/overlay2
```

优先复用现有 `HostConfig.StorageOpt`，实验参数 `dsec.lowerdirs`（仅为建议；
若源码显示有更小、更局部的 per-container plumbing，可采用之并记录原因）。

## 4. 第一版不要过度实现 metadata lifecycle

目标首先是 `docker create/run → external pre-mounted EROFS 真实进入 OverlayFS
lowerdir`。metadata persistence、daemon restart recovery 若非第一次 smoke 的
前置条件，放到后续 lifecycle phase（wip §32）。

## 5. init layer 特别检查

必须实际确认 StorageOpt 是否同时传播给 Docker init layer 与最终 RW layer；
若导致 external lower 插入两次 → 最小修复；若当前源码不存在该路径 →
不要为文档假设而改代码。

## 6. Smoke Test 最低要求

至少：Docker base image + workspace EROFS + 可选 toolkit EROFS。

- **Rootfs composition**：host 上真实 mount 必须形如
  `lowerdir=<toolkit>:<workspace>:<docker-lowers>,upperdir=<docker-upper>,workdir=<docker-work>`
- **Precedence**：同路径冲突时 toolkit > workspace > Docker image
- **Copy-up**：container 内修改 EROFS-origin file 后，Docker UpperDir 出现修改
  版本，而 EROFS image hash 与 mounted content 均不变

## 7. Phase 1A 结束后强制停止

完成后不要继续 Phase 1B。先汇报：

```text
1. 实际 baseline SHA
2. 实际 overlay2 调用链
3. 修改了哪些文件
4. git diff --stat
5. Moby 核心 patch 实际行数
6. 为什么需要这些修改
7. dsec.lowerdirs 的参数传递路径
8. 最终 OverlayFS lowerdir 顺序
9. smoke test 命令和结果
10. copy-up 验证结果
11. 是否发现与论文描述不同的 Moby 行为
12. 下一阶段前仍有哪些风险
```

若 Moby 核心 patch 膨胀到数百行或新增复杂 storage abstraction，
继续前必须解释为什么无法采用更小实现。
