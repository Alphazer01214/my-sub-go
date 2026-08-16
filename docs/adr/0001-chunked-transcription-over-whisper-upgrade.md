# 分块转录取代升级 whisper.cpp 修复重复字幕缺陷

长视频（约 30 分钟起）转录到后半段时反复输出同一句字幕，且与 whisper 模型选择无关。根因是整段音频被一次 `whisper_full` 送入（`chunk_duration_sec` 配置形同虚设），whisper 在 BGM/静音段触发温度回退幻觉重复。我们选择在 Go 侧落地分块转录，而不是升级 vendored whisper.cpp v1.8.4。

## 考虑过的方案

- **升级 whisper.cpp**：能拿到 VAD 时间戳漂移修复（issue #3683 / PR #3711）等上游修复，但需要重编译 `libwhisper.a` 与 ggml 系列静态库（CUDA 13.1 工具链），风险高、验证周期长，且升级本身不直接解决"整段单次识别"的结构问题。
- **落地分块转录（采用）**：按 `chunk_duration_sec=20` / `chunk_overlap_sec=5` 切块，逐块 `whisper_full` + `offset_ms` 绝对时间戳，块间用上一块尾部文本做 `initial_prompt`，重叠区去重，另加幻觉重复守卫。每块 20 秒使温度回退与 VAD 时间映射漂移都无法跨块累积。

## 后果

- 若将来仍观察到时间戳漂移（文字不同但时间戳冻结），再评估升级 whisper.cpp 或关 VAD；分块结构保持不变。
- `chunk_duration_sec` / `chunk_overlap_sec` 从死配置变为真实生效参数。
