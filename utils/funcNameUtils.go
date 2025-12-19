package utils

import (
	"runtime"
	"strings"
)

func GetFuncName(skip int) string {
	pc, _, _, _ := runtime.Caller(skip)
	fullName := runtime.FuncForPC(pc).Name()
	splitted := strings.Split(fullName, ".")
	return strings.Join(splitted[len(splitted)-2:], ".")
}
