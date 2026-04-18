//go:build windows

package gui

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

const (
	fosPickFolders     = 0x00000020
	fosForceFileSystem = 0x00000040
	fosPathMustExist   = 0x00000800
	sigdnFileSysPath   = 0x80058000
	hrCanceled         = 0x800704C7
)

var (
	clsidFileOpenDialog = win.CLSID{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = win.IID{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidIShellItem       = win.IID{0x43826D1E, 0xE718, 0x42EE, [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}

	shell32DLL               = syscall.NewLazyDLL("shell32.dll")
	procSHCreateItemFromPath = shell32DLL.NewProc("SHCreateItemFromParsingName")
)

type iFileDialogVtbl struct {
	QueryInterface      uintptr
	AddRef              uintptr
	Release             uintptr
	Show                uintptr
	SetFileTypes        uintptr
	SetFileTypeIndex    uintptr
	GetFileTypeIndex    uintptr
	Advise              uintptr
	Unadvise            uintptr
	SetOptions          uintptr
	GetOptions          uintptr
	SetDefaultFolder    uintptr
	SetFolder           uintptr
	GetFolder           uintptr
	GetCurrentSelection uintptr
	SetFileName         uintptr
	GetFileName         uintptr
	SetTitle            uintptr
	SetOkButtonLabel    uintptr
	SetFileNameLabel    uintptr
	GetResult           uintptr
	AddPlace            uintptr
	SetDefaultExtension uintptr
	Close               uintptr
	SetClientGuid       uintptr
	ClearClientData     uintptr
	SetFilter           uintptr
}

type iFileDialog struct {
	lpVtbl *iFileDialogVtbl
}

func (d *iFileDialog) Release() uint32 {
	ret, _, _ := syscall.Syscall(
		d.lpVtbl.Release,
		1,
		uintptr(unsafe.Pointer(d)),
		0,
		0,
	)
	return uint32(ret)
}

func (d *iFileDialog) GetOptions(options *uint32) win.HRESULT {
	ret, _, _ := syscall.Syscall(
		d.lpVtbl.GetOptions,
		2,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(options)),
		0,
	)
	return win.HRESULT(ret)
}

func (d *iFileDialog) SetOptions(options uint32) win.HRESULT {
	ret, _, _ := syscall.Syscall(
		d.lpVtbl.SetOptions,
		2,
		uintptr(unsafe.Pointer(d)),
		uintptr(options),
		0,
	)
	return win.HRESULT(ret)
}

func (d *iFileDialog) SetTitle(title *uint16) win.HRESULT {
	ret, _, _ := syscall.Syscall(
		d.lpVtbl.SetTitle,
		2,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(title)),
		0,
	)
	return win.HRESULT(ret)
}

func (d *iFileDialog) SetDefaultFolder(folder *iShellItem) win.HRESULT {
	ret, _, _ := syscall.Syscall(
		d.lpVtbl.SetDefaultFolder,
		2,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(folder)),
		0,
	)
	return win.HRESULT(ret)
}

func (d *iFileDialog) SetFolder(folder *iShellItem) win.HRESULT {
	ret, _, _ := syscall.Syscall(
		d.lpVtbl.SetFolder,
		2,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(folder)),
		0,
	)
	return win.HRESULT(ret)
}

func (d *iFileDialog) Show(owner win.HWND) win.HRESULT {
	ret, _, _ := syscall.Syscall(
		d.lpVtbl.Show,
		2,
		uintptr(unsafe.Pointer(d)),
		uintptr(owner),
		0,
	)
	return win.HRESULT(ret)
}

func (d *iFileDialog) GetResult(result **iShellItem) win.HRESULT {
	ret, _, _ := syscall.Syscall(
		d.lpVtbl.GetResult,
		2,
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(result)),
		0,
	)
	return win.HRESULT(ret)
}

type iShellItemVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	BindToHandler  uintptr
	GetParent      uintptr
	GetDisplayName uintptr
	GetAttributes  uintptr
	Compare        uintptr
}

type iShellItem struct {
	lpVtbl *iShellItemVtbl
}

func (item *iShellItem) Release() uint32 {
	ret, _, _ := syscall.Syscall(
		item.lpVtbl.Release,
		1,
		uintptr(unsafe.Pointer(item)),
		0,
		0,
	)
	return uint32(ret)
}

func (item *iShellItem) GetDisplayName(sigdn uint32, value **uint16) win.HRESULT {
	ret, _, _ := syscall.Syscall(
		item.lpVtbl.GetDisplayName,
		3,
		uintptr(unsafe.Pointer(item)),
		uintptr(sigdn),
		uintptr(unsafe.Pointer(value)),
	)
	return win.HRESULT(ret)
}

func showModernFolderDialog(owner walk.Form, title string, initialDir string) (string, bool, error) {
	if hr := win.OleInitialize(); hr != win.S_OK && hr != win.S_FALSE {
		return "", false, fmt.Errorf("OleInitialize failed: %#x", uint32(hr))
	}
	defer win.OleUninitialize()

	var dialogPtr unsafe.Pointer
	hr := win.CoCreateInstance(
		&clsidFileOpenDialog,
		nil,
		win.CLSCTX_INPROC_SERVER,
		&iidIFileOpenDialog,
		&dialogPtr,
	)
	if win.FAILED(hr) {
		return "", false, fmt.Errorf("CoCreateInstance(FileOpenDialog) failed: %#x", uint32(hr))
	}
	dialog := (*iFileDialog)(dialogPtr)
	defer dialog.Release()

	var options uint32
	hr = dialog.GetOptions(&options)
	if win.FAILED(hr) {
		return "", false, fmt.Errorf("IFileDialog::GetOptions failed: %#x", uint32(hr))
	}
	options |= fosPickFolders | fosForceFileSystem | fosPathMustExist
	hr = dialog.SetOptions(options)
	if win.FAILED(hr) {
		return "", false, fmt.Errorf("IFileDialog::SetOptions failed: %#x", uint32(hr))
	}

	if title != "" {
		titleUTF16, err := syscall.UTF16PtrFromString(title)
		if err != nil {
			return "", false, err
		}
		hr = dialog.SetTitle(titleUTF16)
		if win.FAILED(hr) {
			return "", false, fmt.Errorf("IFileDialog::SetTitle failed: %#x", uint32(hr))
		}
	}

	if initialDir != "" {
		if folderItem, err := shellItemFromPath(initialDir); err == nil {
			defer folderItem.Release()
			_ = dialog.SetDefaultFolder(folderItem)
			_ = dialog.SetFolder(folderItem)
		}
	}

	var ownerHandle win.HWND
	if owner != nil {
		ownerHandle = owner.Handle()
	}

	hr = dialog.Show(ownerHandle)
	if uint32(hr) == hrCanceled {
		return "", false, nil
	}
	if win.FAILED(hr) {
		return "", false, fmt.Errorf("IFileDialog::Show failed: %#x", uint32(hr))
	}

	var resultItem *iShellItem
	hr = dialog.GetResult(&resultItem)
	if win.FAILED(hr) {
		return "", false, fmt.Errorf("IFileDialog::GetResult failed: %#x", uint32(hr))
	}
	if resultItem == nil {
		return "", false, nil
	}
	defer resultItem.Release()

	var pathUTF16 *uint16
	hr = resultItem.GetDisplayName(sigdnFileSysPath, &pathUTF16)
	if win.FAILED(hr) {
		return "", false, fmt.Errorf("IShellItem::GetDisplayName failed: %#x", uint32(hr))
	}
	if pathUTF16 == nil {
		return "", false, nil
	}
	defer win.CoTaskMemFree(uintptr(unsafe.Pointer(pathUTF16)))

	selected := filepath.Clean(win.UTF16PtrToString(pathUTF16))
	if selected == "." || selected == "" {
		return "", false, nil
	}
	return selected, true, nil
}

func shellItemFromPath(path string) (*iShellItem, error) {
	normalized := filepath.Clean(path)
	pathUTF16, err := syscall.UTF16PtrFromString(normalized)
	if err != nil {
		return nil, err
	}

	var item *iShellItem
	hr, _, _ := procSHCreateItemFromPath.Call(
		uintptr(unsafe.Pointer(pathUTF16)),
		0,
		uintptr(unsafe.Pointer(&iidIShellItem)),
		uintptr(unsafe.Pointer(&item)),
	)
	if win.FAILED(win.HRESULT(hr)) {
		return nil, fmt.Errorf("SHCreateItemFromParsingName failed: %#x", uint32(hr))
	}
	if item == nil {
		return nil, fmt.Errorf("SHCreateItemFromParsingName returned nil shell item")
	}
	return item, nil
}
