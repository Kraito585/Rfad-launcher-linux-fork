package main

import (
	"context"
	"encoding/binary"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"

	application "github.com/wailsapp/wails/v3/pkg/application"
)

const (
	iocRead = 2

	evKey = 0x01
	evAbs = 0x03

	btnSouth     = 0x130 // A / Cross
	btnEast      = 0x131 // B / Circle
	btnDpadUp    = 0x220
	btnDpadDown  = 0x221
	btnDpadLeft  = 0x222
	btnDpadRight = 0x223

	absX     = 0x00
	absY     = 0x01
	absRY    = 0x04
	absHat0X = 0x10
	absHat0Y = 0x11

	gpDeadzone     = 0.45
	gpInitialDelay = 350 * time.Millisecond
	gpRepeatRate   = 120 * time.Millisecond
)

type inputAbsInfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

func eviocgbit(ev, length uintptr) uintptr {
	return (iocRead << 30) | (length << 16) | ('E' << 8) | (0x20 + ev)
}

func eviocgabs(abs uintptr) uintptr {
	return (iocRead << 30) | (uintptr(unsafe.Sizeof(inputAbsInfo{})) << 16) | ('E' << 8) | (0x40 + abs)
}

func hasEvBit(bits []byte, bit int) bool {
	idx := bit / 8
	return idx < len(bits) && (bits[idx]&(1<<uint(bit%8))) != 0
}

func isGamepadFd(fd uintptr) bool {
	var keyBits [96]byte
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		eviocgbit(evKey, uintptr(len(keyBits))),
		uintptr(unsafe.Pointer(&keyBits[0])),
	)
	if errno != 0 {
		return false
	}
	return hasEvBit(keyBits[:], btnSouth)
}

func readAbsInfo(fd uintptr, code uint16) (inputAbsInfo, bool) {
	var info inputAbsInfo
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		eviocgabs(uintptr(code)),
		uintptr(unsafe.Pointer(&info)),
	)
	if errno != 0 || info.Maximum <= info.Minimum {
		return info, false
	}
	return info, true
}

func normalizeAxis(val int32, info inputAbsInfo) float64 {
	span := float64(info.Maximum - info.Minimum)
	if span == 0 {
		return 0
	}
	center := float64(info.Maximum+info.Minimum) / 2.0
	return (float64(val) - center) / (span / 2.0)
}

func emitGamepadAction(action string) {
	if app := application.Get(); app != nil {
		app.Event.Emit("gamepad-action", action)
	}
}

func emitGamepadScroll(delta float64) {
	if app := application.Get(); app != nil {
		app.Event.Emit("gamepad-scroll", delta)
	}
}

func startGamepadMonitor(ctx context.Context) {
	var (
		mu     sync.Mutex
		active = make(map[string]struct{})
	)

	scan := func() {
		matches, err := filepath.Glob("/dev/input/event*")
		if err != nil {
			return
		}
		for _, path := range matches {
			mu.Lock()
			_, exists := active[path]
			mu.Unlock()
			if exists {
				continue
			}

			f, err := os.OpenFile(path, os.O_RDONLY, 0)
			if err != nil {
				continue
			}
			if !isGamepadFd(f.Fd()) {
				f.Close()
				continue
			}

			mu.Lock()
			active[path] = struct{}{}
			mu.Unlock()

			slog.Info("Геймпад подключен к лаунчеру", "device", path)

			go func(devPath string, file *os.File) {
				defer func() {
					file.Close()
					mu.Lock()
					delete(active, devPath)
					mu.Unlock()
					slog.Info("Геймпад отключен", "device", devPath)
				}()
				readGamepadLoop(ctx, file)
			}(path, f)
		}
	}

	scan()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scan()
		}
	}
}

func readGamepadLoop(ctx context.Context, f *os.File) {
	done := make(chan struct{})
	defer close(done)

	// Закрываем дескриптор при завершении лаунчера, чтобы разблокировать Read
	go func() {
		select {
		case <-ctx.Done():
			_ = f.Close()
		case <-done:
		}
	}()

	absInfos := make(map[uint16]inputAbsInfo)
	for _, code := range []uint16{absX, absY, absRY} {
		if info, ok := readAbsInfo(f.Fd(), code); ok {
			absInfos[code] = info
		}
	}

	var (
		stateMu    sync.Mutex
		stickX     float64
		stickY     float64
		rightY     float64
		hatX       int32
		hatY       int32
		dpadUp     bool
		dpadDown   bool
		dpadLeft   bool
		dpadRight  bool
		prevDirs   = map[string]bool{}
		nextRepeat = map[string]time.Time{}
	)

	updateDirsLocked := func() {
		now := time.Now()
		cur := map[string]bool{
			"up":    dpadUp || hatY < 0 || stickY < -gpDeadzone,
			"down":  dpadDown || hatY > 0 || stickY > gpDeadzone,
			"left":  dpadLeft || hatX < 0 || stickX < -gpDeadzone,
			"right": dpadRight || hatX > 0 || stickX > gpDeadzone,
		}
		for _, dir := range []string{"up", "down", "left", "right"} {
			if cur[dir] && !prevDirs[dir] {
				emitGamepadAction(dir)
				nextRepeat[dir] = now.Add(gpInitialDelay)
			}
			prevDirs[dir] = cur[dir]
		}
	}

	// Автоповтор при удержании направления и скролл правым стиком
	go func() {
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				stateMu.Lock()
				for _, dir := range []string{"up", "down", "left", "right"} {
					if prevDirs[dir] && !nextRepeat[dir].IsZero() && !now.Before(nextRepeat[dir]) {
						emitGamepadAction(dir)
						nextRepeat[dir] = now.Add(gpRepeatRate)
					}
				}
				if math.Abs(rightY) > gpDeadzone {
					emitGamepadScroll(rightY * 18)
				}
				stateMu.Unlock()
			}
		}
	}()

	const eventSize = 24
	buf := make([]byte, eventSize*16)

	for {
		n, err := f.Read(buf)
		if err != nil {
			return
		}
		for off := 0; off+eventSize <= n; off += eventSize {
			evType := binary.LittleEndian.Uint16(buf[off+16 : off+18])
			evCode := binary.LittleEndian.Uint16(buf[off+18 : off+20])
			evVal := int32(binary.LittleEndian.Uint32(buf[off+20 : off+24]))

			switch evType {
			case evKey:
				switch evCode {
				case btnSouth:
					if evVal == 1 {
						emitGamepadAction("confirm")
					}
				case btnEast:
					if evVal == 1 {
						emitGamepadAction("back")
					}
				case btnDpadUp, btnDpadDown, btnDpadLeft, btnDpadRight:
					stateMu.Lock()
					pressed := evVal != 0
					switch evCode {
					case btnDpadUp:
						dpadUp = pressed
					case btnDpadDown:
						dpadDown = pressed
					case btnDpadLeft:
						dpadLeft = pressed
					case btnDpadRight:
						dpadRight = pressed
					}
					updateDirsLocked()
					stateMu.Unlock()
				}

			case evAbs:
				stateMu.Lock()
				switch evCode {
				case absX:
					if info, ok := absInfos[absX]; ok {
						stickX = normalizeAxis(evVal, info)
					}
				case absY:
					if info, ok := absInfos[absY]; ok {
						stickY = normalizeAxis(evVal, info)
					}
				case absRY:
					if info, ok := absInfos[absRY]; ok {
						rightY = normalizeAxis(evVal, info)
					}
				case absHat0X:
					hatX = evVal
				case absHat0Y:
					hatY = evVal
				}
				updateDirsLocked()
				stateMu.Unlock()
			}
		}
	}
}
