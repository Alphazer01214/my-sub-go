package gui

import (
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"my-sub-go/common/logx"
	"my-sub-go/typedef"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// getRealTimeObj 根据字段类型生成对应控件。
// supportExt: 文件选择对话框的扩展名过滤（逗号分隔，如 ".mp4,.mkv"），空则不过滤。
// onChanged: 值发生变化后的回调（用于保存界面状态），可为 nil。
func getRealTimeObj(w *fyne.Window, fieldType string, value *reflect.Value, supportExt string, onChanged func()) fyne.CanvasObject {
	var obj fyne.CanvasObject
	exts := splitExts(supportExt)
	switch fieldType {
	case "bool":
		b := binding.NewBool()
		if err := b.Set(value.Bool()); err != nil {
			logx.Error(logx.ModuleSystem, "bool 控件绑定失败: %v", err)
		}
		obj = widget.NewCheckWithData("", b)

	case "int":
		b := binding.NewInt()
		if err := b.Set(int(value.Int())); err != nil {
			logx.Error(logx.ModuleSystem, "int 控件绑定失败: %v", err)
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
			logx.Error(logx.ModuleSystem, "textarea 控件绑定失败: %v", err)
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
			logx.Error(logx.ModuleSystem, "lang 控件绑定失败: %v", err)
		}
		entry := widget.NewSelectWithData(typedef.LangOptions, b)
		entry.OnChanged = func(s string) {
			value.SetString(s)
		}
		obj = entry

	case "dir":
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			logx.Error(logx.ModuleSystem, "dir 控件绑定失败: %v", err)
		}
		entry := widget.NewEntryWithData(b)
		entry.OnChanged = func(s string) {
			value.SetString(s)
			if onChanged != nil {
				onChanged()
			}
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
					if onChanged != nil {
						onChanged()
					}
				}
			}, *w)
			setDialogLocation(d, value.String())
			d.Show()
		})
		obj = container.NewGridWithColumns(2, entry, btn)

	case "file":
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			logx.Error(logx.ModuleSystem, "file 控件绑定失败: %v", err)
		}
		entry := widget.NewEntryWithData(b)
		entry.OnChanged = func(s string) {
			value.SetString(s)
			if onChanged != nil {
				onChanged()
			}
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
					if onChanged != nil {
						onChanged()
					}
				}
			}, *w)
			setDialogLocation(d, value.String())
			if len(exts) > 0 {
				d.SetFilter(storage.NewExtensionFileFilter(exts))
			}
			d.Show()
		})
		obj = container.NewGridWithColumns(2, entry, btn)

	default:
		// string
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			logx.Error(logx.ModuleSystem, "string 控件绑定失败: %v", err)
		}
		entry := widget.NewEntryWithData(b)
		entry.OnChanged = func(s string) {
			value.SetString(s)
		}
		obj = entry
	}

	return obj
}

// splitExts 把 ".mp4,.mkv" 拆成 []string，空串返回 nil。
func splitExts(support string) []string {
	if strings.TrimSpace(support) == "" {
		return nil
	}
	parts := strings.Split(support, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// setDialogLocation 把文件/目录对话框定位到当前值所在目录，
// 避免每次都从根目录/工作目录全量扫描（加速对话框弹出）。
func setDialogLocation(d dialog.Dialog, current string) {
	if strings.TrimSpace(current) == "" {
		return
	}
	uri, err := storage.ParseURI("file://" + filepath.ToSlash(filepath.Dir(current)))
	if err != nil {
		return
	}
	l, err := storage.ListerForURI(uri)
	if err != nil {
		return
	}
	if fd, ok := d.(interface{ SetLocation(fyne.ListableURI) }); ok {
		fd.SetLocation(l)
	}
}

func bindObj(fieldType string, value *reflect.Value, bind binding.DataItem, obj fyne.CanvasObject) (binding.DataItem, fyne.CanvasObject) {
	switch fieldType {
	case "bool":
		b := binding.NewBool()
		if err := b.Set(value.Bool()); err != nil {
			logx.Error(logx.ModuleSystem, "bool 控件绑定失败: %v", err)
		}
		bind = b
		obj = widget.NewCheckWithData("", b)

	case "int":
		b := binding.NewInt()
		if err := b.Set(int(value.Int())); err != nil {
			logx.Error(logx.ModuleSystem, "int 控件绑定失败: %v", err)
		}
		bind = b
		obj = widget.NewEntryWithData(binding.IntToString(b))

	case "textarea":
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			logx.Error(logx.ModuleSystem, "textarea 控件绑定失败: %v", err)
		}
		bind = b
		e := widget.NewEntryWithData(b)
		e.MultiLine = true
		obj = e

	case "lang":
		b := binding.NewString()
		if err := b.Set(value.String()); err != nil {
			logx.Error(logx.ModuleSystem, "lang 控件绑定失败: %v", err)
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
			logx.Error(logx.ModuleSystem, "string 控件绑定失败: %v", err)
		}
		bind = b
		obj = widget.NewEntryWithData(b)
	}

	return bind, obj
}

// showErrorDialog 弹错误框，并把同一错误写入系统日志（弹窗错误必须留痕可查）。
func showErrorDialog(w fyne.Window, err error) {
	logx.Error(logx.ModuleSystem, "界面错误弹窗: %v", err)
	dialog.ShowError(err, w)
}

// runInBackground 在后台 goroutine 执行耗时操作，完成后回到 UI 线程恢复控件状态并弹窗。
func runInBackground(w fyne.Window, btn *widget.Button, progress *widget.ProgressBarInfinite, name, successMsg string, fn func() error) {
	btn.Disable()
	progress.Show()
	go func() {
		err := fn()
		fyne.Do(func() {
			if err != nil {
				fyne.LogError(name, err)
				logx.Error(logx.ModuleSystem, "%s 失败: %v", name, err)
				showErrorDialog(w, err)
			} else {
				logx.Info(logx.ModuleSystem, "%s 成功", name)
				dialog.ShowInformation("Success", successMsg, w)
			}
			btn.Enable()
			progress.Hide()
		})
	}()
}
