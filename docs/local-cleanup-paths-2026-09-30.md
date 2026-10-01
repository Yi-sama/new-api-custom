# 本地二开项目清理清单

核查与执行日期：2026-09-30。范围为 `D:\Open source project`。前期美国服务器检查为只读，本次删除阶段没有连接服务器。

**执行结果：第一批四个目录和两处旧缓存已删除，合计 16,598 个文件、2,376,801,209 字节，约 2.21 GiB。此前的权限阻碍已通过正式审批解决；第二批及“暂时保留”目录没有删除。**

## 第一批：已完成整目录删除

当前 `new-api` 和 `sub2api-auto-refill-test` 原目录已保留。发布副本中没有入 Git 的少量文件，在删除前后均已确认在这两个原目录有 SHA-256 相同的副本。

| 完整路径 | 逻辑大小 | 恢复依据 |
| --- | ---: | --- |
| `D:\Open source project\.deploy-staging\push-new-api-custom` | 22.52 MiB | `Yi-sama/new-api-custom` 的 `main`，提交 `2ec10067d5ddb8c4b66352e753a94a31906167ab` |
| `D:\Open source project\.deploy-staging\push-sub2api-custom` | 65.83 MiB | `Yi-sama/sub2api-custom` 的 `main`，提交 `b990204ea82041560bfa3b8aad23ee6998542016` |
| `D:\Open source project\.deploy-staging\push-sub2api-statekit-branch` | 65.90 MiB | `Yi-sama/sub2api-custom` 的 `statekit-v0.2.8-test`，提交 `e956d905f48317af50ad59d6dc1fa7aed373e6dd` |
| `D:\Open source project\V Visa-free\new-api备份` | 空目录 | 已确认没有子项 |

三个发布副本原占用合计 161,737,233 字节，约 154.25 MiB。删除前确认均为独立 Git 仓库，无暂存或未提交改动、stash、关联工作树或目录链接。重新执行 `git ls-remote --heads` 确认远端提交一致。

## 两处旧缓存：已清理

确认没有 Go 构建进程后，仅删除了下面两个目录，未删除整个 `.local-tests`：

```text
D:\Open source project\V Visa-free\new-api\.local-tests\channel-status-go-cache
D:\Open source project\V Visa-free\new-api\.local-tests\channel-status-go-tmp
```

原占用合计 2,215,063,976 字节，约 2.06 GiB。下次构建会重建缓存，耗时可能增加。同级数据库、密钥、备份和测试数据均已保留。

## 删除后核验

- 六个目标目录均不存在。
- 两个原开发仓库的 Git 状态、工作区差异、暂存差异摘要在删除前后一致；比较发生在本次文档更新之前。
- `.local-tests` 中其余 834 个文件的路径、大小、修改时间一致。
- `abc-shared.db`、`image-studio-session-secret` 以及九项保留副本的 SHA-256 一致。
- D 盘可用空间由 873,050,112 字节增至 3,274,838,016 字节，约从 0.81 GiB 增至 3.05 GiB。磁盘空闲变化与逻辑文件大小是不同统计口径。
- 未执行构建、安装依赖、应用测试、停止进程或远程部署；验证针对清理范围和保留文件。

## 第二批：先整理，再退役

### Sub2API 自动补池副本

完整路径：`D:\Open source project\sub2api-auto-refill-test`

主要源码已有 `sub2api-custom/main` 发布快照。十个脚本最初显示 Git blob ID 不同，但进一步逐文件 SHA-256 比对确认当前工作文件与发布副本完全相同，不能据此判定存在十处未保存源码。

退役前需要另存下列内容，不能只依赖远端 Git：

- `docs/screenshots/reasoning-effort-pricing.png`
- `docs/seedance-api.md`
- `skills/sub2api-admin/scripts/sub2api-admin.js`
- `docs/debug/2026-09-25-debug.md`
- `docs/progress/2026-09-25.md`
- `docs/project-notes.md`
- `.deploy-staging/docker-compose.quality-layout-20260926.yml`

上述相对路径均位于该自动补池目录内。本轮没有替用户另存这些文件。若还需要原来的提交历史，应另行保留 Git 历史；发布快照保存源码，不等于保留本地 detached 提交 `d403dde` 的全部历史。

### New API Turnstile 部署副本

完整路径：`D:\Open source project\V Visa-free\new-api-turnstile-deploy`

三个修改过的功能代码文件与主项目、已发布快照完全相同；两个语言文件的差异只是末尾换行。剩余三个未跟踪 Linux 二进制属于旧诊断/修复产物，应先明确是否需要保留作回滚材料。

这个目录是主项目的关联 Git 工作树。应使用 Git 工作树管理流程退役，不应直接粗暴递归删除。其 `web/node_modules.shared-junction` 指向主项目依赖，不能删除链接目标。

## 暂时保留

| 完整路径 | 原因 |
| --- | --- |
| `D:\Open source project\V Visa-free\new-api` | 当前仍有开发工作；检查时有 341 个状态条目。今天的盈利功能部署不能用 9 月 28 日的 Git 快照代表。另有本地数据库和运行数据。 |
| `D:\Open source project\V Visa-free\new-api-rc28` | 68 个改动路径中，50 个与发布快照不同或不存在于快照中；还有三个本地数据库。 |
| `D:\Open source project\V Visa-free\new-api-rc28-runtime-channel-health` | 没有 Git 元数据；包含独有 `mock_upstream.py`、数据库、会话导出和日志，未证明有其他完整副本。 |
| `D:\Open source project\sub2api-quality-test` | 16 个源码改动路径与 StateKit 发布快照不一致，包括质量检测接口、前端和数据库迁移。 |
| `D:\Open source project\sub2api-v0.2.8-statekit-test` | 95 个修改/新增源码路径与已发布快照一致，但保存着 `sub2api-quality-test` 的 Git 工作树元数据，不能单独删除。还有未入 Git 的项目笔记。 |
| `D:\Open source project\sub2api-v0.2.7` | 有本地源码改动，且是两个 Responses 部署工作树的 Git 主仓库。 |
| `D:\Open source project\sub2api-v0.2.7-statekit` | Git 元数据不完整，并有部署数据、PostgreSQL 和 Redis 数据目录。 |

参考源码仓库、监控项目、服务器备份和其他个人项目不在本批删除清单内。

## 美国服务器核查

- 主机：`156.238.252.115`，实际主机名 `ser021360749320`。仅使用只读 SSH、Docker 状态和挂载检查，没有构建、安装、重启、推送或删除服务器文件。
- 检查时 `new-api`、`old-new-api`、`sub2api`、`sub2api-test-20260923` 均健康。
- 当前 New API 二进制位于 `/opt/proxy/newapi/releases/20260930-profitability-3b37cfbd/new-api`，不是从本机 D 盘挂载。
- Sub2API 主服务使用服务器 `/opt/sub2api/data`；测试服务使用 `/opt/sub2api-test-20260923/data`。本地目录清理不应触及这两个服务器路径。
- Sub2API 镜像没有可核对的源码 revision 标签；不能仅凭“容器运行正常”证明某个本地目录已被完整上传到 Git。
- 检查时总内存 7,951 MiB，可用 4,776 MiB；Swap 2,047 MiB，已用约 50 MiB；根盘可用约 12 GiB。

验证状态：第一批和旧缓存清理已完成，文件系统与保留项核验通过；第二批暂未执行；本次删除阶段没有改动或重启美国服务器。
