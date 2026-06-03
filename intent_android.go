//go:build android

package main

/*
*/
import "C"

//export intentURINotify
func intentURINotify(uri *C.char) {
	if uri != nil {
		goURI := C.GoString(uri)
		LogD("intent: URI " + goURI)
		select {
		case uriFromIntent <- goURI:
		default:
			LogD("intent: URI channel full, dropping")
		}
	}
}

//export intentTextNotify
func intentTextNotify(text *C.char) {
	if text != nil {
		goText := C.GoString(text)
		LogD("intent: text received")
		select {
		case textFromIntent <- goText:
		default:
			LogD("intent: text channel full, dropping")
		}
	}
}
