// main.go

package main

/*
#cgo CFLAGS: -I${SRCDIR}/dependencies/whisper.cpp/include
#cgo CFLAGS: -I${SRCDIR}/dependencies/whisper.cpp/ggml/include
#cgo LDFLAGS: -L${SRCDIR}/lib -lwhisper -lggml -lggml-base -lggml-cpu -lggml-cuda
#cgo LDFLAGS: -L"C:/Program Files/NVIDIA GPU Computing Toolkit/CUDA/v13.1/lib/x64" -lcudart -lcublas -lcuda
#include "whisper.h"
*/
import "C"

import (
	"my-sub-go/common/logx"
	"my-sub-go/gui"
	"my-sub-go/typedef"
	"os"

	"fyne.io/fyne/v2/app"
)

func main() {
	// 日志系统最先初始化；GUI 尚未创建时致命错误用 MessageBox + 日志文件呈现。
	_ = logx.Init()
	defer logx.Close()
	logx.Info(logx.ModuleSystem, "程序启动")

	fail := func(step string, err error) {
		logx.Error(logx.ModuleSystem, "%s失败: %v", step, err)
		logx.MessageBox("MyGoAutoSub 启动失败", step+"失败：\n"+err.Error())
		os.Exit(1)
	}

	cm := typedef.NewConfigManager(typedef.ConfigPath)
	if err := cm.Init(); err != nil {
		fail("加载配置文件 config/conf.json", err)
	}

	var cvt typedef.Converter
	var ts typedef.Transcriber
	var tl typedef.TranslatorAPI
	cpm, err := typedef.NewComponentManager(cm.Cfg, &cvt, &ts, &tl)
	if err != nil {
		fail("初始化组件", err)
	}

	a := app.NewWithID("com.example.my-sub-go")
	var instance = gui.NewInstance(a)
	if err := instance.Init(cm, cpm); err != nil {
		fail("初始化界面", err)
	}
	instance.Run()

	logx.Info(logx.ModuleSystem, "程序退出")
}
