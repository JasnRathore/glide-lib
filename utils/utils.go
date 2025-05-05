package utils

import (
	"reflect"
	"runtime"
	"strings"
)

func FuncToString(fn interface{}) string {
	fullName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
    shortName := fullName[strings.LastIndex(fullName, ".")+1:]
	return shortName
}

