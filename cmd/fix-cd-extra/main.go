// fix-cd-extra shrinks the Central Directory extra field for .so entries in an APK.
//
// Problem: reproducible F-Droid builds fail with APK Signature Scheme v3
// CHUNKED_SHA256 digest mismatch because the CD extra field for .so files
// differs between the CI reference binary and the F-Droid-built APK.
//
// Root cause: the CI runner (Ubuntu) and F-Droid build server (Debian 13 trixie)
// ship different builds of Info-ZIP zip 3.0 that behave differently when
// rewriting the central directory:
//
//   - Ubuntu zip 3.0 (used by CI): "zip -d --out" writes a minimal CD extra
//     field (0-3 bytes) for all entries, discarding the large alignment padding
//     from the local header extra.
//
//   - Debian 13 trixie zip 3.0-15 (used by F-Droid): "zip -d --out" copies the
//     local header extra field to the CD extra field (+4 bytes), producing a
//     ~3977-byte CD extra for .so entries instead of the expected 1 byte.
//     (The exact value depends on .so alignment padding, which varies per ABI.)
//
// This tool patches the CD extra for .so entries from ~3977 bytes to 1 byte (0x00),
// matching the CI reference binary exactly.
//
// Usage after "zip -d crocson.apk 'META-INF/*' --out crocson-arm.apk":
//
//	go run ./cmd/fix-cd-extra crocson-arm.apk
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: fix-cd-extra <apk>\n")
		os.Exit(1)
	}
	d, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	eocd := -1
	for i := len(d) - 22; i >= 0; i-- {
		if d[i] == 'P' && d[i+1] == 'K' && d[i+2] == 5 && d[i+3] == 6 {
			eocd = i
			break
		}
	}
	if eocd < 0 {
		panic("EOCD not found")
	}

	cdOff := int(binary.LittleEndian.Uint32(d[eocd+16:]))
	n := int(binary.LittleEndian.Uint16(d[eocd+10:]))

	var ncd []byte
	p := cdOff
	fixed := 0
	for i := 0; i < n; i++ {
		nl := int(binary.LittleEndian.Uint16(d[p+28:]))
		el := int(binary.LittleEndian.Uint16(d[p+30:]))
		cl := int(binary.LittleEndian.Uint16(d[p+32:]))
		nm := string(d[p+46 : p+46+nl])

		e := make([]byte, 46+nl+el+cl)
		copy(e, d[p:p+46+nl+el+cl])

		if strings.HasSuffix(nm, ".so") && el > 1 {
			binary.LittleEndian.PutUint16(e[30:], 1)
			e[46+nl] = 0
			e = append(e[:46+nl+1], e[46+nl+el:]...)
			fixed++
		}

		ncd = append(ncd, e...)
		p += 46 + nl + el + cl
	}

	nd := make([]byte, 0, len(d))
	nd = append(nd, d[:cdOff]...)
	nd = append(nd, ncd...)
	nd = append(nd, d[eocd:]...)

	newEOCD := cdOff
	binary.LittleEndian.PutUint32(nd[newEOCD+12:], uint32(len(ncd)))

	if err := os.WriteFile(os.Args[1], nd, 0); err != nil {
		panic(err)
	}
	fmt.Printf("fixed %d .so entries (CD_extra -> 1 byte)\n", fixed)
}
