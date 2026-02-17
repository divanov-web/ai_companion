//go:build windows

package handy

import (
	"context"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

// Обёртки для функций, которых может не быть в lxn/win
var (
	user32                          = syscall.NewLazyDLL("user32.dll")
	procAddClipboardFormatListener  = user32.NewProc("AddClipboardFormatListener")
	procRemoveClipboardFormatListen = user32.NewProc("RemoveClipboardFormatListener")
	procRegisterHotKey              = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey            = user32.NewProc("UnregisterHotKey")
)

type winImpl struct{}

func newWinListener() (winListener, error) { return &winImpl{}, nil }

func (w *winImpl) run(ctx context.Context, clipOut chan<- Event, hotkeyOut chan<- Event) {
	// UI/WinAPI должен жить в закрепленном системном потоке
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	className := syscall.StringToUTF16Ptr("HandyHiddenWindowClass")

	// Регистрация класса окна
	var wc win.WNDCLASSEX
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = syscall.NewCallback(func(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
		switch msg {
		case win.WM_HOTKEY:
			// Ctrl+Enter
			select {
			case hotkeyOut <- Event{Type: EventCtrlEnter, At: time.Now()}:
			default:
			}
			return 0
		case win.WM_CLIPBOARDUPDATE:
			if txt, ok := readClipboardText(); ok {
				select {
				case clipOut <- Event{Type: EventClipboardChanged, Text: txt, At: time.Now()}:
				default:
				}
			}
			return 0
		case win.WM_DESTROY:
			win.PostQuitMessage(0)
			return 0
		}
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	})
	wc.HInstance = win.GetModuleHandle(nil)
	wc.HCursor = win.LoadCursor(0, (*uint16)(unsafe.Pointer(uintptr(win.IDC_ARROW))))
	wc.LpszClassName = className
	if win.RegisterClassEx(&wc) == 0 {
		// возможно, уже зарегистрирован — пробуем продолжить
	}

	// Создаём скрытое окно
	hwnd := win.CreateWindowEx(
		0,
		className,
		syscall.StringToUTF16Ptr("HandyHiddenWindow"),
		0,
		0, 0, 0, 0, // x, y, width, height
		0, // parent
		0, // menu
		wc.HInstance,
		nil,
	)
	if hwnd == 0 {
		return
	}

	// Регистрируем прослушивание буфера
	_ = addClipboardFormatListener(hwnd)

	// Регистрируем глобальный хоткей Ctrl+Enter
	const hotkeyID = 1
	const MOD_CONTROL = 0x0002
	const VK_RETURN = 0x0D
	_ = registerHotKey(hwnd, hotkeyID, MOD_CONTROL, VK_RETURN)

	// Цикл сообщений до отмены контекста
	msg := new(win.MSG)
	// Параллельно следим за ctx и закрываем окно
	done := make(chan struct{}, 1)
	go func() {
		<-ctx.Done()
		win.PostMessage(hwnd, win.WM_CLOSE, 0, 0)
		done <- struct{}{}
	}()

	for {
		r := win.GetMessage(msg, 0, 0, 0)
		if r == 0 || r == -1 { // WM_QUIT или ошибка
			break
		}
		win.TranslateMessage(msg)
		win.DispatchMessage(msg)
		select {
		case <-done:
			break
		default:
		}
	}

	// Очистка
	_ = unregisterHotKey(hwnd, 1)
	_ = removeClipboardFormatListener(hwnd)
	win.DestroyWindow(hwnd)
}

func addClipboardFormatListener(hwnd win.HWND) bool {
	if procAddClipboardFormatListener.Find() != nil {
		return false
	}
	r, _, _ := procAddClipboardFormatListener.Call(uintptr(hwnd))
	return r != 0
}

func removeClipboardFormatListener(hwnd win.HWND) bool {
	if procRemoveClipboardFormatListen.Find() != nil {
		return false
	}
	r, _, _ := procRemoveClipboardFormatListen.Call(uintptr(hwnd))
	return r != 0
}

func registerHotKey(hwnd win.HWND, id int32, modifiers uint32, vk uint32) bool {
	if procRegisterHotKey.Find() != nil {
		return false
	}
	r, _, _ := procRegisterHotKey.Call(uintptr(hwnd), uintptr(id), uintptr(modifiers), uintptr(vk))
	return r != 0
}

func unregisterHotKey(hwnd win.HWND, id int32) bool {
	if procUnregisterHotKey.Find() != nil {
		return false
	}
	r, _, _ := procUnregisterHotKey.Call(uintptr(hwnd), uintptr(id))
	return r != 0
}

func readClipboardText() (string, bool) {
	if win.IsClipboardFormatAvailable(win.CF_UNICODETEXT) == false {
		return "", false
	}
	if win.OpenClipboard(0) == false {
		return "", false
	}
	defer win.CloseClipboard()
	h := win.HGLOBAL(win.GetClipboardData(win.CF_UNICODETEXT))
	if h == 0 {
		return "", false
	}
	p := win.GlobalLock(h)
	if p == nil {
		return "", false
	}
	defer win.GlobalUnlock(h)
	// Считать нуль-терминированную UTF-16 строку
	u16 := (*[1 << 20]uint16)(p) // ограничение 1М элементов
	var n int
	for n = 0; n < len(u16) && u16[n] != 0; n++ {
	}
	if n == 0 {
		return "", true
	}
	return syscall.UTF16ToString(u16[:n]), true
}
