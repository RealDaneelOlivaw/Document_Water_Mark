//go:build windows

package gui

import (
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

func centerMainWindowOnPrimaryWorkArea(mw *walk.MainWindow) {
	if mw == nil {
		return
	}
	b := mw.BoundsPixels()
	if b.Width <= 0 || b.Height <= 0 {
		return
	}
	hwnd := mw.Handle()
	if hwnd == 0 {
		return
	}
	var mi win.MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if !win.GetMonitorInfo(win.MonitorFromWindow(hwnd, win.MONITOR_DEFAULTTOPRIMARY), &mi) {
		return
	}
	work := mi.RcWork
	workX := int(work.Left)
	workY := int(work.Top)
	workW := int(work.Right - work.Left)
	workH := int(work.Bottom - work.Top)
	if workW <= 0 || workH <= 0 {
		return
	}

	x := workX
	if b.Width < workW {
		x = workX + (workW-b.Width)/2
	}
	y := workY
	if b.Height < workH {
		y = workY + (workH-b.Height)/2
	}

	if b.Width <= workW {
		switch {
		case x < workX:
			x = workX
		case x+b.Width > workX+workW:
			x = workX + workW - b.Width
		}
	}
	if b.Height <= workH {
		switch {
		case y < workY:
			y = workY
		case y+b.Height > workY+workH:
			y = workY + workH - b.Height
		}
	}

	_ = mw.SetBoundsPixels(walk.Rectangle{X: x, Y: y, Width: b.Width, Height: b.Height})
}
