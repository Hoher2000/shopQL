package utils

import (
	"runtime"
)

func GetFuncName(skip int) string {
	pc, _, _, _ := runtime.Caller(skip)
	return runtime.FuncForPC(pc).Name()
	//splitted := strings.Split(fullName, ".")
	//return splitted[len(splitted)-1]
}
