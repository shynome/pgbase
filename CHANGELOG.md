# Changelog

## [0.0.2] - 2026-05-24

- 添加 `EnableConcurrentWrites` 辅助函数以便快速启用并发写入
- 优化: polyglot v0.4.1 修复了 [drop_index 丢失分号](https://github.com/tobilg/polyglot/issues/206) 的问题
- 优化: 现在可以单独设置 Cache 和 polyglot.Client 了

## [0.0.1] - 2026-05-23

成功实现了多写, 通过将 pocketbase 的存储后端由 sqlite 切换到 postgres
