package typedef

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// StatePath 界面状态文件（记住上次的媒体文件/输出目录/语言选择），与 conf.json 同目录。
const StatePath = "config/state.json"

// AppState 界面状态：跨启动记住上次的选择。
type AppState struct {
	Workflow struct {
		MediaPath    string `json:"media_path"`
		SubtitlePath string `json:"subtitle_path"`
		SubtitleDir  string `json:"subtitle_dir"`
		Lang         Lang   `json:"lang"`
		Mode         string `json:"mode"`
	} `json:"workflow"`
	Converter struct {
		VideoPath string `json:"video_path"`
		AudioDir  string `json:"audio_dir"`
	} `json:"converter"`
}

// LoadState 读取界面状态；文件不存在或损坏时返回零值（不报错）。
func LoadState(path string) *AppState {
	s := &AppState{}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, s)
	return s
}

// Save 保存界面状态；目录不存在则创建。
func (s *AppState) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
