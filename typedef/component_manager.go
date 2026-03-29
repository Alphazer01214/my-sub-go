package typedef

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type ComponentManager struct {
	Cfg   *Config
	Cvt   *Converter
	Ts    *Transcriber
	TlAPI *TranslatorAPI
}

func NewComponentManager(cfg *Config, comps ...interface{}) *ComponentManager {
	var cm = &ComponentManager{
		Cfg: cfg,
	}
	cm.Init(cfg, comps...)
	return cm
}

func (cm *ComponentManager) Init(cfg *Config, comps ...interface{}) {
	for _, comp := range comps {
		switch comp.(type) {
		case *Converter:
			cm.Cvt = NewConverter(cfg)
			fmt.Println("[main window] converter mounted")

		case *Transcriber:
			cm.Ts = NewTranscriber(cfg)
			fmt.Println("[main window] transcriber mounted")

		case *TranslatorAPI:
			cm.TlAPI = NewTranslatorAPI(cfg)
			fmt.Println("[main window] translator mounted")

		default:
			fmt.Println("[main window] unknown component")
		}
	}
}

func (cm *ComponentManager) RunVideoTranscriber(vPath string, saveSRTDir string, lang string) (*Subtitle, error) {
	// Pipeline
	ext := filepath.Ext(vPath)
	bs := strings.TrimSuffix(filepath.Base(vPath), ext)
	dir := filepath.Dir(vPath)
	tmpFileName := bs + ".wav"
	tmpFilePath := filepath.Join(dir, tmpFileName)
	//jsonSubPath := filepath.Join(dir, bs+".json")
	//srtSubPath := filepath.Join(dir, bs+".srt")
	err := cm.Cvt.GetVideoWavFile(vPath, dir, cm.Cvt.CvtArgs)
	if err != nil {
		return nil, err
	}

	if err := cm.Cvt.GetWavTmpFile(vPath, tmpFilePath); err != nil {
		return nil, err
	}

	sub, err := cm.Ts.GetSRTSubtitleFileFromAudio(tmpFilePath, saveSRTDir, lang)
	if err != nil {
		return nil, err
	}

	if err := os.Remove(tmpFilePath); err != nil {
		return nil, err
	}

	return sub, nil
}

func (cm *ComponentManager) RunAudioTranscriber(aPath string, saveSRTDir string, lang string) (*Subtitle, error) {
	// Pipeline
	sub, err := cm.Ts.GetSRTSubtitleFileFromAudio(aPath, saveSRTDir, lang)
	if err != nil {
		return nil, err
	}
	return sub, err
}

func (cm *ComponentManager) RunTranslatorAPIPipeline(mediaPath string, saveSRTDir string) error {
	// Pipeline
	ext := filepath.Ext(mediaPath)
	base := filepath.Base(mediaPath)
	if slices.Contains(AudioType, ext) {
		sub, err := cm.RunAudioTranscriber(mediaPath, saveSRTDir, cm.Cfg.Whisper.SrcLang)
		if err != nil {
			return err
		}
		res, err := cm.TlAPI.Translate(sub)
		if err != nil {
			return err
		}

		return res.SaveToFile(filepath.Join(saveSRTDir, strings.TrimSuffix(base, ext)+"-bilingual.srt"))
	}

	if slices.Contains(VideoType, ext) {
		sub, err := cm.RunVideoTranscriber(mediaPath, saveSRTDir, cm.Cfg.Whisper.SrcLang)
		if err != nil {
			return err
		}
		res, err := cm.TlAPI.Translate(sub)
		if err != nil {
			return err
		}
		return res.SaveToFile(filepath.Join(saveSRTDir, base+"-bilingual.srt"))
	}
	return fmt.Errorf("[main window] unsupported media type %s", ext)
}
