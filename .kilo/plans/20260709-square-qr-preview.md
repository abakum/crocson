# Square Preview for QR Scanner

## Goal
Make QR scanner preview square by center-cropping camera frames to the minimum dimension after rotation.

## Context
- Camera resolutions: typically 640x480, 480x320, 320x240 (max 640x480 constraint in GoNativeActivity.java:363)
- Frames are rotated based on device orientation (qrRot rotations of 90° CW)
- Both preview and QR decoding use the same grayscale image after rotation (qr_camera.go:111,119)
- Current issue: `canvas.ImageFillContain` preserves frame proportions, causing tall preview

## Implementation Plan

### 1. Add cropCenterSquare function
**Location**: qr_camera.go, after `rotateGray90cw` (line 95)

```go
// cropCenterSquare crops grayscale buffer to center square by minimum side.
// Returns new buffer of size min(w,h) × min(w,h) with center content.
func cropCenterSquare(src []byte, w, h int) (dst []byte, nw, nh int) {
    side := min(w, h)
    if w == h {
        // Already square - return original
        return src, w, h
    }
    // Calculate center offsets
    xoff := (w - side) / 2
    yoff := (h - side) / 2
    // Allocate square buffer
    dst = make([]byte, side*side)
    // Copy center region
    for r := 0; r < side; r++ {
        rowSrc := src[(yoff+r)*w+xoff : (yoff+r)*w+xoff+side]
        copy(dst[r*side:r*side+side], rowSrc)
    }
    return dst, side, side
}
```

### 2. Apply crop after rotation
**Location**: qr_camera.go:110, in `qrDecodeLoop`

```go
for i := 0; i < qrRot; i++ {
    y, w, h = rotateGray90cw(y, w, h)
}
y, w, h = cropCenterSquare(y, w, h)  // NEW: crop to square
gray := &image.Gray{Pix: y, Stride: w, Rect: image.Rect(0, 0, w, h)}
```

## Data Flow
1. Java → Go: NV21 frame (w×h)
2. Rotate: 0-3 times (qrRot) based on sensor orientation
3. **NEW**: Crop to min(w,h) × min(w,h) center square
4. Create image.Gray for both preview and QR decoding
5. Display in qrPreview (canvas.ImageFillContain on square = square)
6. QR decode from same image

## Constraints & Assumptions
- **No minimum crop size**: Real camera resolutions (320-480px min side) are sufficient for QR decoding
- **Go 1.21+**: Using `min()` builtin (if older Go, use `func min(a,b int) int { if a<b{return a}; return b }`)
- **Memory**: Same allocation pattern as rotateGray90cw - acceptable for QR scanning
- **Center crop**: QR codes are typically framed in center, so center crop doesn't reduce detection reliability

## Validation
1. Preview should be square (not tall)
2. QR codes should still decode successfully
3. Rotate + crop should handle all orientations correctly
4. Memory usage should not significantly increase (similar to rotation)

## Risk Assessment
- **Low**: Crop size (320-480px) is sufficient for QR decoding
- **Low**: Center crop is standard approach for QR scanning
- **Low**: Memory allocation per frame is acceptable (already allocating for rotation)

## Files Modified
- qr_camera.go: Add `cropCenterSquare` function, call it in `qrDecodeLoop`