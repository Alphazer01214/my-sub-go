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
	"fmt"
	"my-sub-go/gui"
	"my-sub-go/typedef"
	"os"

	"fyne.io/fyne/v2/app"
)

func main() {
	cm := typedef.NewConfigManager(typedef.ConfigPath)
	if err := cm.Init(); err != nil {
		panic(err)
	}
	var cvt typedef.Converter
	var ts typedef.Transcriber
	var tl typedef.TranslatorAPI
	cpm := typedef.NewComponentManager(cm.Cfg, &cvt, &ts, &tl)
	a := app.NewWithID("com.example.my-sub-go")
	var instance = gui.NewInstance(a)
	if err := instance.Init(cm, cpm); err != nil {
		fmt.Println("Error initializing instance:", err)
		os.Exit(1)
	}
	instance.Run()

	fmt.Println("finished")
}
