package handler

import (
	"runtime"
	"strconv"
)

// CallerPC returns the program counter for the caller at the given skip depth.
// Returns 0 if caller information is unavailable.
func CallerPC(skip int) uintptr {
	var pcs [1]uintptr
	if runtime.Callers(skip, pcs[:]) > 0 {
		return pcs[0]
	}
	return 0
}

// PCToLocation converts a program counter to file:line format.
func PCToLocation(pc uintptr) string {
	if pc == 0 {
		return ""
	}
	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	return frame.File + ":" + strconv.Itoa(frame.Line)
}
