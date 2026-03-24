package internal

//func RunPipeline(t *Transcriber, cvt *Converter, path string, config common.Config) error {
//	ext := filepath.Ext(path)
//	bs := strings.TrimSuffix(filepath.Base(path), ext)
//	dir := filepath.Dir(path)
//	tmpFileName := bs + "_TMP.wav"
//	tmpFilePath := filepath.Join(dir, tmpFileName)
//	jsonSubPath := filepath.Join(dir, bs+".json")
//	srtSubPath := filepath.Join(dir, bs+".srt")
//
//	if err := cvt.GetWavTmpFile(path, tmpFilePath); err != nil {
//		return err
//	}
//	sub, err := t.Process(tmpFilePath)
//	if err != nil {
//		return err
//	}
//	PrintSub(sub)
//	*sub, err = Translate(*sub, config.LLMAPI)
//	if err != nil {
//		return err
//	}
//	if err := sub.SaveToFile(srtSubPath); err != nil {
//		return err
//	}
//	data, err := json.MarshalIndent(sub, "", "  ")
//	if err != nil {
//		return err
//	}
//	if err := os.WriteFile(jsonSubPath, data, 0644); err != nil {
//		return err
//	}
//	if err := os.Remove(tmpFilePath); err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func PrintSub(sub *common.Subtitle) {
//	fmt.Println("[Subtitle] ASR result:")
//	for _, segment := range sub.Segments {
//		fmt.Printf("[%d] %s\n", segment.Index, segment.Text)
//	}
//}
