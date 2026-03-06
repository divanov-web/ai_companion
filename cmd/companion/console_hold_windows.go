//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func holdConsoleIfNeeded() {
	if !isStandaloneConsoleWindow() {
		return
	}

	fmt.Print("\nНажмите Enter, чтобы закрыть окно... ")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

func isStandaloneConsoleWindow() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleProcessList := kernel32.NewProc("GetConsoleProcessList")

	ids := make([]uint32, 2)
	r, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafePointerToFirst(ids)),
		uintptr(len(ids)),
	)

	return r <= 1
}

func unsafePointerToFirst(ids []uint32) unsafe.Pointer {
	if len(ids) == 0 {
		return nil
	}
	return unsafe.Pointer(&ids[0])
}
