# 下一个版本草稿

候选 Tag：`v0.7.0`（普通语义化版本，见 [`docs/RELEASING.md`](../RELEASING.md)）。

上一个预发行版的冻结说明位于 [`v0.6.0-clawgod.2.md`](v0.6.0-clawgod.2.md)。
请继续使用中文优先、英文摘要补充的结构。

## 待发布变更

- 移除可选 ClawGod 运行时；`claude-kiro` 只使用官方 Claude Code。
- 安装目录与环境变量改名为 `claude-kiro` / `CLAUDE_KIRO_*`（不兼容旧路径）。
- 发布 Tag 改为普通语义化版本 `vX.Y.Z`。
- 保留粘贴图片与工具返回图片；请求上限提高到 32 MiB。
- 安装程序同时创建 `ck` 快捷命令，与 `claude-kiro` 行为一致。

## 发布验证清单

- [ ] 指定唯一的候选 Tag。
- [ ] 更新 `CHANGELOG.md` 和版本化发布说明。
- [ ] 本地检查与 GitHub Linux/Windows CI 全部通过。
- [ ] 只推送单个 Tag，不使用 `git push --tags`。
- [ ] 核验全部制品、`checksums.txt` 和 GitHub Release 页面。
