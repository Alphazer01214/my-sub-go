package gui

import (
	"my-sub-go/typedef"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func getRealTimeObj(w *fyne.Window, fieldType string, value *reflect.Value) fyne.CanvasObject {
	var obj fyne.CanvasObject
	switch fieldType {
	case "bool":
		b := binding.NewBool()
		if err := b.Set(value.Bool()); err != nil {
			panic(err)
		}
		obj = widget.NewCheckWithData("", b)

	case "int":
		b := binding.NewInt()
		if err := b.Set(int(value.Int())); err != nil {
			panic(err)
		}
		entry := widget.NewEntryWithData(binding.IntToString(b))
		entry.OnChanged = func(s string) {
			i, err := strconv.Atoi(s)
			if err != nil {
				//dialog.ShowError(err, *w)
				return
			}
			value.SetInt(int64(i))
		}
		obj = entry

	case "textarea":
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			panic(err)
		}
		e := widget.NewEntryWithData(b)
		e.MultiLine = true
		e.OnChanged = func(s string) {
			value.SetString(s)
		}
		obj = e

	case "lang":
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			panic(err)
		}
		entry := widget.NewSelectWithData(typedef.LangOptions, b)
		entry.OnChanged = func(s string) {
			value.SetString(s)
		}
		obj = entry

	case "dir":
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			panic(err)
		}
		entry := widget.NewEntryWithData(b)
		entry.OnChanged = func(s string) {
			value.SetString(s)
		}
		//entry.Disable()
		entry.TextStyle = fyne.TextStyle{Bold: true}
		btn := widget.NewButton("选择目录", func() {
			d := dialog.NewFolderOpen(func(d fyne.ListableURI, err error) {
				if err == nil && d != nil {
					//current, _ := b.Get()
					value.SetString(d.Path())
					err := b.Set(d.Path())
					if err != nil {
						return
					}
				}
			}, *w)
			//if currentDir := value.String(); currentDir != "" {
			//	path, _ := storage.ParseURI(filepath.Dir(currentDir))
			//	listable, _ := storage.ListerForURI(path)
			//	d.SetLocation(listable)
			//}

			d.Show()
		})
		obj = container.NewGridWithColumns(2, entry, btn)

	case "file":
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			panic(err)
		}
		entry := widget.NewEntryWithData(b)
		entry.OnChanged = func(s string) {
			value.SetString(s)
		}
		entry.TextStyle = fyne.TextStyle{Bold: true}
		//entry.Disable()
		btn := widget.NewButton("选择文件", func() {
			d := dialog.NewFileOpen(func(f fyne.URIReadCloser, err error) {
				if err == nil && f != nil {
					value.SetString(f.URI().Path())
					err := b.Set(f.URI().Path())
					if err != nil {
						return
					}
				}
			}, *w)
			//if currentPath := value.String(); currentPath != "" {
			//	path, _ := storage.ParseURI(filepath.Dir(currentPath))
			//	listable, _ := storage.ListerForURI(path)
			//	d.SetLocation(listable)
			//}
			d.Show()
		})
		obj = container.NewGridWithColumns(2, entry, btn)

	default:
		// string
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			panic(err)
		}
		entry := widget.NewEntryWithData(b)
		entry.OnChanged = func(s string) {
			value.SetString(s)
		}
		obj = entry
	}

	return obj
}
func bindObj(fieldType string, value *reflect.Value, bind binding.DataItem, obj fyne.CanvasObject) (binding.DataItem, fyne.CanvasObject) {
	switch fieldType {
	case "bool":
		b := binding.NewBool()
		if err := b.Set(value.Bool()); err != nil {
			panic(err)
		}
		bind = b
		obj = widget.NewCheckWithData("", b)

	case "int":
		b := binding.NewInt()
		if err := b.Set(int(value.Int())); err != nil {
			panic(err)
		}
		bind = b
		obj = widget.NewEntryWithData(binding.IntToString(b))

	case "textarea":
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			panic(err)
		}
		bind = b
		e := widget.NewEntryWithData(b)
		e.MultiLine = true
		obj = e

	case "lang":
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			panic(err)
		}
		bind = b
		obj = widget.NewSelectWithData(typedef.LangOptions, b)

	//case "dir":
	//	b := binding.NewString()
	//	if err := b.Set(value.String()); err != nil {
	//		panic(err)
	//	}
	//	bind = b
	//	entry := widget.NewEntryWithData(b)
	//	btn := widget.NewButton("选择目录", func() {
	//		dialog.NewFolderOpen(func(d fyne.ListableURI, err error) {
	//			if err == nil && d != nil {
	//				//current, _ := b.Get()
	//				b.Set(d.Path())
	//			}
	//		}, *w).Show()
	//	})
	//	obj = container.NewGridWithColumns(2, entry, btn)
	//	return bind, obj
	//
	//case "file":
	//	b := binding.NewString()
	//	if err := b.Set(value.String()); err != nil {
	//		panic(err)
	//	}
	//	bind = b
	//	entry := widget.NewEntryWithData(b)
	//	btn := widget.NewButton("选择文件", func() {
	//		fyne.Do(func() { // ✅ 延迟到下一帧执行
	//			d := dialog.NewFileOpen(func(f fyne.URIReadCloser, err error) {
	//				if err == nil && f != nil {
	//					b.Set(f.URI().Path())
	//				}
	//			}, *w)
	//
	//			if currentPath, err := b.Get(); err == nil && currentPath != "" {
	//				//dir := filepath.Dir(currentPath)
	//				//if uri, err := storage.ParseURI("file://" + dir); err == nil {
	//				//	d.SetLocation(uri)
	//				//}
	//			}
	//			d.Show()
	//		})
	//	})
	//	obj = container.NewGridWithColumns(2, entry, btn)
	//	return bind, obj

	default:
		// string
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			panic(err)
		}
		bind = b
		obj = widget.NewEntryWithData(b)
	}

	return bind, obj
}

// restartApp 重启当前程序
func restartApp() {
	// 获取当前可执行文件路径
	exe, err := os.Executable()
	if err != nil {
		return
	}

	// 根据操作系统不同处理
	switch runtime.GOOS {
	case "windows":
		// Windows 上使用 cmd /c start 来启动新进程
		cmd := exec.Command("cmd", "/c", "start", exe)
		err := cmd.Start()
		if err != nil {
			return
		}
	case "darwin", "linux":
		// Unix-like 系统使用 syscall.Exec
		err = syscall.Exec(exe, os.Args, os.Environ())
		if err != nil {
			return
		}
	}

	// 退出当前程序
	os.Exit(0)
}
