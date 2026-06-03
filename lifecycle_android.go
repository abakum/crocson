//go:build android

package main

/*
*/
import "C"

var lifecycleFromJava = make(chan string, 10)

//export lifecycleEventNotify
func lifecycleEventNotify(event *C.char) {
	goEvent := C.GoString(event)
	LogD("lifecycle: " + goEvent)
	select {
	case lifecycleFromJava <- goEvent:
	default:
		LogD("lifecycle: channel full, dropping " + goEvent)
	}
}
