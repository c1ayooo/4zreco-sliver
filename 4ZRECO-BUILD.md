# 4zreco 二改构建说明

本仓库是 BishopFox/sliver v1.7.7 的二开源码树（GPL-3.0），**不独立编译**——
作为主仓库（4zreco）的 `sliver/` 子树并入主 module，包路径 `4zreco/sliver/...`。

交叉编译工具链（zig/go 归档，~209MB）**不入库**：由主仓库构建入口在编译前
按 `scripts/sliver-toolchain.lock` 从上游 BishopFox/sliver v1.7.7 在线下载并
sha256 校验，编译完成后自动清理（`SLIVER_KEEP_TOOLCHAIN=1` 可留存）。

- 拉取/构建入口：主仓库 `./bootstrap.sh` 或 `./run.sh`（缺树自动 clone 本仓库）
- 手动补工具链：`scripts/sliver-toolchain.sh ensure`
- 查看状态：`scripts/sliver-toolchain.sh status`
