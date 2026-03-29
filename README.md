# MyGoSubtitle - 自动翻译工具

MyGoSubtitle 是一款基于 AI 的字幕自动生成和翻译工具，使用 Whisper 语音识别模型和大语言模型（LLM）实现视频/音频的自动转录和翻译。支持多种语言和媒体格式，可生成高质量的双语字幕。

## 功能特性
- 🎯 **语音识别**: 基于 Whisper.cpp 的高精度语音转文字
- 🌍 **多语言支持**: 支持英语、中文、日语、韩语、法语、德语等多种语言
- 🤖 **AI 翻译**: 支持 OpenAI、Ollama 等大语言模型API进行翻译
- 🎬 **多媒体支持**: 支持 MP4、MKV、AVI、MOV、WMV、FLV、WebM 等视频格式
- ⚡ **CUDA 加速**: 支持 NVIDIA GPU 加速，大幅提升处理速度
- 🔊 **VAD 检测**: 智能语音活动检测，提高字幕分割准确性
- 💻 **图形界面**: 简洁易用的 Fyne GUI 界面
- 🔧 **灵活配置**: 支持自定义模型路径、翻译参数等
- 注意：LLama暂不支持

## 部署
### 配置文件
在 config 文件夹下有 `conf.json`，可以进入软件配置。

**配置说明：**
- **FFmpeg**: 配置音视频处理参数
  - `binary_path`: FFmpeg 可执行文件路径
  - `sample_rate`: 音频采样率（默认 16000Hz）
  - `audio_codec`: 音频编解码器（默认 pcm_s16le）

- **Whisper**: 语音识别配置
  - `binary_path`: Whisper CLI 工具路径
  - `model_path`: Whisper 模型文件路径
  - `vad_path`: VAD 模型路径（可选）
  - `src_lang`: 源语言代码
  - `chunk_duration_sec`: 音频分块时长（秒）
  - `chunk_overlap_sec`: 分块重叠时长（秒）
  - `threads`: 处理线程数

- **Llama**: 本地翻译模型配置
  - `binary_path`: Llama CLI 工具路径
  - `model_path`: Llama 模型文件路径
  - `prompt_template`: 翻译提示词模板
  - `src_lang/tgt_lang`: 源语言和目标语言

- **LLM API**: 翻译API配置
  - `provider`: 服务提供商（openai, ollama）
  - `base_url`: API 基础 URL
  - `model_name`: 使用的模型名称
  - `api_key`: API 访问密钥
  - `ref_window`: 参考上下文大小
  - `process_window`: 一次翻译句数

- **全局配置**
  - `cuda`: 是否启用 CUDA 加速
  - `default_output_dir`: 默认输出目录
### cuda配置
把ggml.dll, ggml-base.dll, ggml-cpu.dll, ggml-cuda.dll 复制到可执行文件目录下。这些可以在nvidia compute的官网下载。
没有这些文件，软件无法启动。

### Whisper.cpp go binding
在 dependencies 下执行 `git clone https://github.com/ggerganov/whisper.cpp.git`

在 `whisper.cpp/bindings/go/whisper.go` 修改：
```cpp
#cgo CFLAGS: -I../../include
#cgo CFLAGS: -I../../ggml/include
#cgo LDFLAGS: -lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++
#cgo linux LDFLAGS: -fopenmp
#cgo darwin LDFLAGS: -lggml-metal -lggml-blas
#cgo darwin LDFLAGS: -framework Accelerate -framework Metal -framework Foundation -framework CoreGraphics
#include <whisper.h>
#include <stdlib.h>
```

## 使用
### 基本使用流程
1. **启动程序**: 运行编译后的可执行文件 `my-sub-go.exe`
2. **配置参数**: 在设置界面中配置以下参数：
   - FFmpeg 路径：FFmpeg 可执行文件路径
   - Whisper 模型路径：语音识别模型文件路径
   - LLM API 配置：选择翻译服务提供商（OpenAI/Ollama 等）
   - 源语言和目标语言：设置音频源语言和翻译目标语言
3. **导入媒体文件**: 选择需要处理的视频或音频文件（支持 mp4、mkv、avi、mov、wmv、flv、webm 等格式）
4. **生成字幕**: 
   - 点击"转录"按钮生成语音识别字幕
   - 点击"翻译"按钮翻译字幕内容
5. **导出字幕**: 将生成的双语字幕保存为 SRT 格式

### 使用示例
#### 示例 1：日语视频翻译成中文
1. 打开软件，进入设置界面
2. 配置 Whisper：`src_lang` 设置为 `ja`（日语）
3. 配置 LLM API：`src_lang` 设置为 `ja`，`tgt_lang` 设置为 `zh`（中文）
4. 选择日语视频文件 `anime.mp4`
5. 点击"转录"生成日文字幕
6. 点击"翻译"生成中日双语字幕
7. 保存为 `anime_bilingual.srt`

### 支持的媒体格式
- **视频格式**: `.mp4`, `.wmv`, `.avi`, `.mkv`, `.mov`, `.flv`, `.webm`
- **音频格式**: `.mp3`, `.wav`, `.m4a`, `.aac`, `.ogg`, `.flac`

### 支持的语言
支持多种语言的语音识别和翻译，包括：
- 英语 (en)
- 中文 (zh)
- 日语 (ja)
- 韩语 (ko)
- 法语 (fr)
- 德语 (de)
- 西班牙语 (es)
- 俄语 (ru)
- 自动检测 (auto)

### 高级功能
- **VAD 语音活动检测**: 自动检测语音片段，提高字幕分割准确性
- **上下文感知翻译**: 使用参考窗口保持翻译的连贯性
- **批量处理**: 支持一次处理多个媒体文件
- **CUDA 加速**: 支持 NVIDIA GPU 加速，大幅提升处理速度

## 技术架构
### 核心组件
- **Whisper.cpp**: OpenAI Whisper 语音识别模型的 C++ 实现，用于语音转文字
- **LLM 翻译**: 支持多种大语言模型 API（OpenAI、DeepSeek、Ollama 等）进行字幕翻译
- **FFmpeg**: 音视频处理工具，用于提取音频流
- **Fyne GUI**: Go 语言跨平台图形界面框架

### 项目结构
```
my-sub-go/
├── common/           # 通用工具和辅助函数
├── config/          # 配置文件
├── dependencies/    # 第三方依赖（whisper.cpp, llama.cpp, ffmpeg）
├── gui/             # 图形用户界面
├── internal/        # 内部实现模块
├── lib/             # 编译库文件
├── models/          # AI 模型文件
│   ├── translator/  # 翻译模型
│   ├── vad/         # VAD 模型
│   └── whisper/     # Whisper 模型
├── test/            # 测试文件
├── typedef/         # 类型定义和接口
└── main.go          # 程序入口
```

## 常见问题
### CUDA 配置问题
如果遇到 CUDA 相关错误，请确保：
1. 已安装 NVIDIA 显卡和 CUDA Toolkit
2. 已下载并复制正确的 DLL 文件到可执行文件目录
3. 在配置文件中启用 CUDA 选项

### 模型下载
#### Whisper 模型
Whisper 模型可以从以下地址下载：
- **官方仓库**: [Hugging Face - Whisper](https://huggingface.co/ggerganov/whisper.cpp)
- **推荐模型**:
  - `ggml-tiny.bin` - 速度最快，准确度较低
  - `ggml-base.bin` - 平衡速度和准确性
  - `ggml-medium.bin` - 推荐使用，准确度高（默认）
  - `ggml-large-v3.bin` - 准确度最高，速度较慢

将下载的模型文件放入 `models/whisper/` 目录

#### VAD 模型（可选）
语音活动检测模型用于提高字幕分割准确性：
- **下载地址**: [Silero VAD](https://huggingface.co/ggerganov/whisper.cpp/blob/main/ggml-silero-v6.2.0.bin)
- 将模型文件放入 `models/vad/` 目录

### LLM API 配置
使用在线 LLM 服务需要：
1. 注册相应服务平台获取 API Key
2. 在配置文件中填写正确的 Base URL 和 Model Name
3. 使用 Ollama 本地部署时无需 API Key

## 开发
### 构建要求
- Go 1.21+
- CMake 3.14+
- GCC/MinGW
- CUDA Toolkit 13.1+ (可选，用于 GPU 加速)

### 编译步骤
```bash
# 克隆项目
git clone https://github.com/your-username/my-sub-go.git
cd my-sub-go

# 安装依赖
go mod download

# 编译
go build -o my-sub-go.exe
```

## 性能优化建议

### 提高翻译质量
1. **增加参考窗口**: 增大 `ref_window` 以提供更多上下文
2. **调整处理窗口**: 减小 `process_window` 可提高单句翻译质量
3. **优化提示词**: 在 `prompt_template` 中添加特定领域说明
4. **使用更强的模型**: 选择更大的 LLM 模型（如 DeepSeek-V3、GPT-4）

## 许可证
本项目采用 MIT 许可证

## 贡献
欢迎贡献代码、报告问题或提出建议！

### 贡献方式
1. **提交 Issue**: 遇到问题或有功能建议时，请创建 Issue
2. **Pull Request**: 
   - Fork 本仓库
   - 创建特性分支 (`git checkout -b feature/AmazingFeature`)
   - 提交更改 (`git commit -m 'Add some AmazingFeature'`)
   - 推送到分支 (`git push origin feature/AmazingFeature`)
   - 开启 Pull Request
3. **改进文档**: 帮助完善 README 或使用文档

### 开发规范
- 遵循 Go 语言标准格式
- 为新增功能编写测试用例
- 保持代码注释清晰
- 使用有意义的变量和函数名

## 致谢
感谢以下开源项目：
- [Whisper.cpp](https://github.com/ggerganov/whisper.cpp) - Whisper C++ 实现
- [FFmpeg](https://ffmpeg.org/) - 音视频处理工具
- [Fyne](https://fyne.io/) - Go GUI 框架
- [Eino](https://github.com/cloudwego/eino) - 大模型应用开发框架