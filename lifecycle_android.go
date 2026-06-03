//go:build android

package main

/*
*/
import "C"

//export lifecycleEventNotify
func lifecycleEventNotify(event *C.char) {
	LogD("lifecycle: " + C.GoString(event))
}
