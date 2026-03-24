package common

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FFmpegConfig FFmpeg配置
type FFmpegConfig struct {
	BinaryPath string `json:"binary_path"` // FFmpeg可执行文件路径
	SampleRate int    `json:"sample_rate"` // 音频采样率
	AudioCodec string `json:"audio_codec"` // 音频编解码器
}

// WhisperConfig Whisper配置
type WhisperConfig struct {
	BinaryPath       string `json:"binary_path"` // Whisper可执行文件路径
	ModelPath        string `json:"model_path"`  // Whisper模型路径
	VADPath          string `json:"vad_path"`
	SrcLang          string `json:"src_lang"`           // 源语言
	ChunkDurationSec int    `json:"chunk_duration_sec"` // 音频分块时长(秒)
	ChunkOverlapSec  int    `json:"chunk_overlap_sec"`  // 音频分块重叠时长(秒)
	Threads          int    `json:"threads"`
}

// LlamaConfig Llama配置
type LlamaConfig struct {
	BinaryPath     string `json:"binary_path"`     // Llama可执行文件路径
	ModelPath      string `json:"model_path"`      // Llama模型路径
	PromptTemplate string `json:"prompt_template"` // 提示词模板
	SrcLang        string `json:"src_lang"`        // 源语言
	TgtLang        string `json:"tgt_lang"`        // 目标语言
}

// LLMAPIConfig LLM API配置
type LLMAPIConfig struct {
	Provider       string `json:"provider"`
	BaseURL        string `json:"base_url"`        // API基础URL
	ModelName      string `json:"model_name"`      // 模型名称
	APIKey         string `json:"api_key"`         // API密钥
	SrcLang        string `json:"src_lang"`        // 源语言
	TgtLang        string `json:"tgt_lang"`        // 目标语言
	PromptTemplate string `json:"prompt_template"` // 提示词模板
	RefWindow      int    `json:"ref_window"`      // 参考上下文大小
	ProcessWindow  int    `json:"process_window"`  // 一次翻译几句
}

// Config 主配置结构体
type Config struct {
	FFmpeg           FFmpegConfig  `json:"ffmpeg"`             // FFmpeg配置
	Whisper          WhisperConfig `json:"whisper"`            // Whisper配置
	Llama            LlamaConfig   `json:"llama"`              // Llama配置
	LLMAPI           LLMAPIConfig  `json:"llm_api"`            // LLM API配置
	CUDA             bool          `json:"cuda"`               // 是否启用CUDA
	DefaultOutputDir string        `json:"default_output_dir"` // 默认输出目录
	I18n             map[string]string
}

// LoadConfig 加载配置文件
func LoadConfig() (Config, error) {
	//err := godotenv.Load()
	//if err != nil {
	//	return Config{}, err
	//}
	// 获取配置文件路径
	configPath := filepath.Join("config", "conf.json")

	// 读取配置文件内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}

	// 解析JSON配置
	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return Config{}, err
	}
	//config.LLMAPI.APIKey = os.Getenv("LLM_API_KEY")
	//config.LLMAPI.BaseURL = os.Getenv("LLM_BASE_URL")
	//config.LLMAPI.ModelName = os.Getenv("LLM_MODEL_NAME")

	//i18nPath := filepath.Join("config", "i18n.json")

	return config, nil
}

func (c *Config) Save() error {
	// 获取配置文件路径
	configPath := filepath.Join("config", "conf.json")

	// 将配置结构体转换为JSON
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// 写入配置文件
	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}
