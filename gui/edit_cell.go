package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// cellEntry 包装 widget.Entry，补上焦点丢失回调与 Esc 取消。
type cellEntry struct {
	*widget.Entry
	onFocusLost func()
	onCancel    func()
}

func (e *cellEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onFocusLost != nil {
		e.onFocusLost()
	}
}

func (e *cellEntry) KeyDown(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		if e.onCancel != nil {
			e.onCancel()
			return
		}
	}
	e.Entry.KeyDown(ev)
}

// editCell 字幕表格单元格：默认显示标签，双击进入编辑。
// 单行文本按 Enter 提交；多行文本在失去焦点时提交；Esc 取消。
type editCell struct {
	widget.BaseWidget
	stack  *fyne.Container
	label  *widget.Label
	single *cellEntry // 单行编辑（Enter 提交）
	multi  *cellEntry // 多行编辑（失焦提交）

	win      fyne.Window
	row, col int
	editable bool
	editing  bool
	active   *cellEntry

	onCommit    func(row, col int, text string)
	onTap       func(row, col int)
	onBeginEdit func()
	onEndEdit   func()
}

func newEditCell(win fyne.Window, onCommit func(row, col int, text string), onTap func(row, col int)) *editCell {
	c := &editCell{win: win, onCommit: onCommit, onTap: onTap}
	c.ExtendBaseWidget(c)
	c.label = widget.NewLabel("")
	c.label.Truncation = fyne.TextTruncateEllipsis
	c.single = &cellEntry{Entry: widget.NewEntry()}
	c.single.onFocusLost = c.commit
	c.single.onCancel = c.cancel
	c.single.OnSubmitted = func(string) { c.commit() }
	c.multi = &cellEntry{Entry: widget.NewEntry()}
	c.multi.MultiLine = true
	c.multi.Wrapping = fyne.TextWrapBreak
	c.multi.onFocusLost = c.commit
	c.multi.onCancel = c.cancel
	c.single.Hide()
	c.multi.Hide()
	c.stack = container.NewStack(c.label, c.single, c.multi)
	return c
}

func (c *editCell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.stack)
}

// setLabel 更新显示文本；bold 用于表头。若正在编辑则隐式结束（如滚动出视野）。
func (c *editCell) setLabel(text string, bold bool) {
	if c.editing {
		c.editing = false
		c.active = nil
		c.single.Hide()
		c.multi.Hide()
		if c.onEndEdit != nil {
			c.onEndEdit()
		}
	}
	c.label.SetText(text)
	c.label.TextStyle = fyne.TextStyle{Bold: bold}
	c.label.Show()
	c.stack.Refresh()
}

func (c *editCell) setEditable(v bool) {
	c.editable = v
}

func (c *editCell) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap(c.row, c.col)
	}
}

func (c *editCell) DoubleTapped(*fyne.PointEvent) {
	c.beginEdit()
}

func (c *editCell) beginEdit() {
	if !c.editable || c.editing {
		return
	}
	c.editing = true
	if strings.Contains(c.label.Text, "\n") {
		c.active = c.multi
	} else {
		c.active = c.single
	}
	c.active.SetText(c.label.Text)
	c.single.Hide()
	c.multi.Hide()
	c.label.Hide()
	c.active.Show()
	c.stack.Refresh()
	if c.onBeginEdit != nil {
		c.onBeginEdit()
	}
	c.win.Canvas().Focus(c.active)
}

func (c *editCell) commit() {
	if !c.editing {
		return
	}
	c.editing = false
	text := strings.TrimSpace(c.active.Text)
	c.active.Hide()
	c.win.Canvas().Focus(nil)
	c.stack.Refresh()
	if c.onEndEdit != nil {
		c.onEndEdit()
	}
	if c.onCommit != nil {
		c.onCommit(c.row, c.col, text)
	}
}

func (c *editCell) cancel() {
	if !c.editing {
		return
	}
	c.editing = false
	c.active.Hide()
	c.win.Canvas().Focus(nil)
	c.label.Show()
	c.stack.Refresh()
	if c.onEndEdit != nil {
		c.onEndEdit()
	}
}
