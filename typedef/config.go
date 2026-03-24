package typedef

import (
	"encoding/json"
	"os"
)

// define: `json:"binary_path" label: "name" description: "xxx" type: "file/dir/int/string/bool/lang/textarea" placeholder: "default" options: "a,b,c" support_ext: ".mp4,.wmv"`

var LangOptions = []string{"en", "zh", "ja", "ko", "fr", "de", "es", "ru", "auto"}
var VideoType = []string{".mp4", ".wmv", ".avi", ".mkv", ".mov"}
var AudioType = []string{".mp3", ".wav", ".m4a", ".aac", ".ogg", ".flac"}
var ConfigPath = "config/conf.json"

// GUI types

type Metadata struct {
	Name        string // struct name
	Label       string
	Description string
	Type        string
	Placeholder string
	Options     []string
	SupportExt  string
	Group       string
	Path        string // example: FFmpeg.BinaryPath
}

type FFmpegConfig struct {
	BinaryPath string `json:"binary_path" label:"FFmpeg路径" description:"FFmpeg可执行文件路径" type:"file" placeholder:"dependencies/ffmpeg/bin/ffmpeg.exe" support_ext:".exe"`
	SampleRate int    `json:"sample_rate" label:"采样率" description:"音频采样率(Hz)" type:"int" placeholder:"16000"`
	AudioCodec string `json:"audio_codec" label:"音频编码" description:"音频编解码器格式" type:"string" placeholder:"pcm_s16le"` // 音频编解码器
}

// WhisperConfig Whisper配置
type WhisperConfig struct {
	BinaryPath       string `json:"binary_path" label:"Whisper路径" description:"Whisper可执行文件路径" type:"file" placeholder:"dependencies/whisper.cpp/bin/whisper-cli.exe" support_ext:".exe"`
	ModelPath        string `json:"model_path" label:"模型路径" description:"Whisper模型文件路径" type:"file" placeholder:"models/whisper/ggml-medium.bin" support_ext:".bin"`
	VADPath          string `json:"vad_path" label:"VAD路径" description:"语音活动检测模型路径" type:"file" placeholder:"models/vad/ggml-silero-v6.2.0.bin" support_ext:".bin"`
	SrcLang          string `json:"src_lang" label:"源语言" description:"音频源语言" type:"lang" placeholder:"en"`
	ChunkDurationSec int    `json:"chunk_duration_sec" label:"切片时长" description:"音频分块时长(秒)" type:"int" placeholder:"20"` // 音频分块时长(秒)
	ChunkOverlapSec  int    `json:"chunk_overlap_sec" label:"切片重叠" description:"音频分块重叠时长(秒)" type:"int" placeholder:"5"` // 音频分块重叠时长(秒)
	Threads          int    `json:"threads" label:"线程数" description:"处理线程数量" type:"int" placeholder:"8"`
}

// LlamaConfig Llama配置
type LlamaConfig struct {
	BinaryPath     string `json:"binary_path" label:"Llama路径" description:"Llama可执行文件路径" type:"file" placeholder:"dependencies/llama.cpp/bin/llama.exe" support_ext:".exe"`           // Llama可执行文件路径
	ModelPath      string `json:"model_path" label:"模型路径" description:"Llama模型文件路径" type:"file" placeholder:"models/translator/translategemma-4b-it.Q4_K_M.gguf" support_ext:".gguf"` // Llama模型路径
	PromptTemplate string `json:"prompt_template" label:"提示词模板" description:"翻译提示词模板" type:"textarea" placeholder:"You are an expert translator."`                                    // 提示词模板
	SrcLang        string `json:"src_lang" label:"源语言" description:"翻译源语言" type:"lang" placeholder:"en"`                                                                              // 源语言
	TgtLang        string `json:"tgt_lang" label:"目标语言" description:"翻译目标语言" type:"lang" placeholder:"zh"`
}

// LLMAPIConfig LLM API配置
type LLMAPIConfig struct {
	Provider       string `json:"provider" label:"提供商" description:"LLM服务提供商" type:"string" placeholder:"deepseek" options:"openai,deepseek,qwen,claude"`
	BaseURL        string `json:"base_url" label:"Base URL" description:"API基础URL" type:"string" placeholder:"https://api.deepseek.com"`
	ModelName      string `json:"model_name" label:"模型名称" description:"使用的模型名称" type:"string" placeholder:"deepseek-chat"`
	APIKey         string `json:"api_key" label:"API Key" description:"API访问密钥" type:"string" placeholder:"sk-xxxxxxxx"`
	SrcLang        string `json:"src_lang" label:"源语言" description:"翻译源语言" type:"lang" placeholder:"en"`
	TgtLang        string `json:"tgt_lang" label:"目标语言" description:"翻译目标语言" type:"lang" placeholder:"zh"`
	PromptTemplate string `json:"prompt_template" label:"提示词模板" description:"翻译提示词模板" type:"textarea" placeholder:"You are translating a Trance electronic music production tutorial."`
	RefWindow      int    `json:"ref_window" label:"参考窗口" description:"参考上下文大小" type:"int" placeholder:"2"`
	ProcessWindow  int    `json:"process_window" label:"处理窗口" description:"一次翻译句数" type:"int" placeholder:"8"`
}

// Config 主配置结构体
type Config struct {
	FFmpeg           FFmpegConfig      `json:"ffmpeg" group:"FFmpeg"`
	Whisper          WhisperConfig     `json:"whisper" group:"Whisper"`                                                                  // Whisper配置
	Llama            LlamaConfig       `json:"llama" group:"Llama"`                                                                      // Llama配置
	LLMAPI           LLMAPIConfig      `json:"llm_api" group:"LLMAPI"`                                                                   // LLM API配置
	CUDA             bool              `json:"cuda" label:"启用CUDA" description:"是否启用CUDA加速" type:"bool" group:"Global"`                  // 是否启用CUDA
	DefaultOutputDir string            `json:"default_output_dir" label:"默认输出目录" description:"默认字幕输出目录" type:"dir" placeholder:"output"` // 默认输出目录
	I18n             map[string]string `json:"I18n"`
}

type ConfigManager struct {
	Cfg  *Config
	path string
	//widgetMap   map[string]fyne.CanvasObject
	//metadataMap map[string]Metadata
}

func NewConfigManager(path string) *ConfigManager {
	return &ConfigManager{
		path: path,
		//widgetMap:   make(map[string]fyne.CanvasObject),
		//metadataMap: make(map[string]Metadata),
	}
}

func (cm *ConfigManager) Init() error {
	// read config file
	data, err := os.ReadFile("config/conf.json")
	if err != nil {
		return err
	}
	cm.Cfg = &Config{}
	err = json.Unmarshal(data, cm.Cfg)
	if err != nil {
		return err
	}

	// get env variables
	//if err := godotenv.Load(); err != nil {
	//	return err
	//}
	//url := os.Getenv("LLM_BASE_URL")
	//api := os.Getenv("LLM_API_KEY")
	//name := os.Getenv("LLM_MODEL_NAME")

	return nil
}

func (cm *ConfigManager) Save() error {
	data, err := json.MarshalIndent(cm.Cfg, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(cm.path, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

// Reset 将配置重置为配置文件中的默认值
func (cm *ConfigManager) Reset() error {
	configPath := cm.path
	if configPath == "" {
		configPath = ConfigPath
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	newCfg := &Config{}
	if err := json.Unmarshal(data, newCfg); err != nil {
		return err
	}

	cm.Cfg = newCfg
	return nil
}
