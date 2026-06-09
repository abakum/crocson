// cmd/apks filters an .apks file to keep only the last uncompressed .so variant,
// patches targeting to SDK >= 23, and writes a new .apks with matching files.
//
// Usage:
//
//	go run ./cmd/apks [-o output.apks] input.apks
package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	var inPath, outPath string
	args := os.Args[1:]
	for len(args) > 0 {
		switch args[0] {
		case "-o":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "usage: apks [-o output.apks] input.apks")
				os.Exit(1)
			}
			outPath = args[1]
			args = args[2:]
		default:
			if inPath == "" {
				inPath = args[0]
			}
			args = args[1:]
		}
	}
	if inPath == "" {
		fmt.Fprintln(os.Stderr, "usage: apks [-o output.apks] input.apks")
		os.Exit(1)
	}
	if outPath == "" {
		ext := filepath.Ext(inPath)
		outPath = inPath[:len(inPath)-len(ext)] + "2" + ext
	}

	if err := processAPKS(inPath, outPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func processAPKS(inPath, outPath string) error {
	r, err := zip.OpenReader(inPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", inPath, err)
	}
	defer r.Close()

	var tocData []byte
	fileData := map[string][]byte{}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %s in zip: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("read %s: %w", f.Name, err)
		}
		if f.Name == "toc.pb" {
			tocData = data
		} else {
			fileData[f.Name] = data
		}
	}

	fmt.Printf("Input: %s (%d files)\n", inPath, len(fileData)+1)
	for name := range fileData {
		fmt.Printf("  %s (%d bytes)\n", name, len(fileData[name]))
	}

	paths := map[string]bool{}
	newToc, err := fixTOC(tocData, paths)
	if err != nil {
		return fmt.Errorf("fix toc.pb: %w", err)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer out.Close()

	w := zip.NewWriter(out)

	wf, err := w.Create("toc.pb")
	if err != nil {
		return err
	}
	wf.Write(newToc)

	for p := range paths {
		data, ok := fileData[p]
		if !ok {
			return fmt.Errorf("file %s referenced in toc not found in archive", p)
		}
		wf, err := w.Create(p)
		if err != nil {
			return err
		}
		if _, err := wf.Write(data); err != nil {
			return err
		}
		fmt.Printf("  %s (%d bytes)\n", p, len(data))
	}

	if err := w.Close(); err != nil {
		return err
	}

	fi, _ := out.Stat()
	fmt.Printf("\nOutput: %s (%d bytes)\n", outPath, fi.Size())
	return nil
}

func fixTOC(toc []byte, paths map[string]bool) ([]byte, error) {
	fields := parseFields(toc)

	var bestVariant []byte
	var bestVN int
	for _, f := range fields {
		if f.num != 1 {
			continue
		}
		vn := getVariantNumber(f.data)
		uncomp := getUncompressedNativeLibs(f.data)
		fmt.Printf("  variant_number=%d uncompressed_native_libraries=%v\n", vn, uncomp)
		if uncomp && vn > bestVN {
			bestVariant = f.data
			bestVN = vn
		}
	}

	if bestVariant == nil {
		fmt.Println("  -> no uncompressed variant found, skipping")
		return toc, nil
	}

	fmt.Printf("  -> keeping variant_number=%d\n", bestVN)

	fixed := patchVariantTargeting(bestVariant)
	collectPaths(fixed, paths)

	var newTopFields []fieldEntry
	for _, f := range fields {
		if f.num != 1 {
			newTopFields = append(newTopFields, f)
		}
	}
	newTopFields = append(newTopFields, fieldEntry{num: 1, wireType: 2, data: fixed})

	return serializeFields(newTopFields), nil
}

func patchVariantTargeting(variant []byte) []byte {
	fields := parseFields(variant)
	for i, f := range fields {
		if f.num != 1 || f.wireType != 2 {
			continue
		}
		fields[i] = fieldEntry{num: 1, wireType: 2, data: patchSdkTargeting(f.data)}
	}
	return serializeFields(fields)
}

func patchSdkTargeting(vt []byte) []byte {
	newTargeting := serializeFields([]fieldEntry{
		{num: 1, wireType: 2, data: serializeFields([]fieldEntry{
			{num: 1, wireType: 0, val: 23},
		})},
	})
	return newTargeting
}

func collectPaths(variant []byte, paths map[string]bool) {
	for _, f := range parseFields(variant) {
		if f.num != 2 {
			continue
		}
		for _, af := range parseFields(f.data) {
			if af.num != 2 {
				continue
			}
			for _, df := range parseFields(af.data) {
				if df.num == 2 {
					paths[string(df.data)] = true
				}
			}
		}
	}
}

func getVariantNumber(variant []byte) int {
	for _, f := range parseFields(variant) {
		if f.num == 3 {
			return int(f.val)
		}
	}
	return 0
}

func getUncompressedNativeLibs(variant []byte) bool {
	for _, f := range parseFields(variant) {
		if f.num == 4 {
			for _, sf := range parseFields(f.data) {
				if sf.num == 2 {
					return sf.val != 0
				}
			}
		}
	}
	return false
}

type fieldEntry struct {
	num      uint64
	wireType int
	val      uint64
	data     []byte
}

func parseFields(msg []byte) []fieldEntry {
	var fields []fieldEntry
	pos := 0
	for pos < len(msg) {
		tag, next := decodeVarint(msg, pos)
		if next == pos {
			break
		}
		pos = next
		fieldNum := tag >> 3
		wireType := int(tag & 0x7)

		switch wireType {
		case 0:
			v, next := decodeVarint(msg, pos)
			if next == pos {
				break
			}
			pos = next
			fields = append(fields, fieldEntry{num: fieldNum, wireType: 0, val: v})
		case 2:
			length, next := decodeVarint(msg, pos)
			if next == pos {
				break
			}
			pos = next
			data := make([]byte, length)
			copy(data, msg[pos:pos+int(length)])
			pos += int(length)
			fields = append(fields, fieldEntry{num: fieldNum, wireType: 2, data: data})
		case 1:
			pos += 8
			fields = append(fields, fieldEntry{num: fieldNum, wireType: 1})
		case 5:
			pos += 4
			fields = append(fields, fieldEntry{num: fieldNum, wireType: 5})
		default:
			break
		}
	}
	return fields
}

func serializeFields(fields []fieldEntry) []byte {
	var buf bytes.Buffer
	for _, f := range fields {
		tag := (f.num << 3) | uint64(f.wireType)
		buf.Write(encodeVarint(tag))
		switch f.wireType {
		case 0:
			buf.Write(encodeVarint(f.val))
		case 2:
			buf.Write(encodeVarint(uint64(len(f.data))))
			buf.Write(f.data)
		case 1:
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], 0)
			buf.Write(b[:])
		case 5:
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], 0)
			buf.Write(b[:])
		}
	}
	return buf.Bytes()
}

func decodeVarint(data []byte, pos int) (uint64, int) {
	var result uint64
	shift := 0
	for pos < len(data) {
		b := data[pos]
		result |= uint64(b&0x7F) << shift
		pos++
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return result, pos
}

func encodeVarint(v uint64) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}
