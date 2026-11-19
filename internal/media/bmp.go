package media

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"math/bits"
)

// decodeBMP reads what the Windows clipboard puts there: uncompressed 24 and 32
// bit bitmaps, with or without the bit-field masks a BI_BITFIELDS header adds.
// golang.org/x/image/bmp refuses those masks, and every screenshot carries them,
// so the decoding happens here.
func decodeBMP(data []byte) (image.Image, error) {
	if len(data) < 54 || data[0] != 'B' || data[1] != 'M' {
		return nil, errors.New("not a bitmap")
	}
	offset := int(binary.LittleEndian.Uint32(data[10:]))
	header := int(binary.LittleEndian.Uint32(data[14:]))
	if header < 40 || len(data) < 14+header {
		return nil, errors.New("short bitmap header")
	}
	width := int(int32(binary.LittleEndian.Uint32(data[18:])))
	height := int(int32(binary.LittleEndian.Uint32(data[22:])))
	depth := int(binary.LittleEndian.Uint16(data[28:]))
	compression := binary.LittleEndian.Uint32(data[30:])
	if width <= 0 || height == 0 {
		return nil, errors.New("bitmap has no size")
	}
	if depth != 24 && depth != 32 {
		return nil, errors.New("unsupported bitmap depth")
	}
	topDown := height < 0
	if topDown {
		height = -height
	}
	if compression != 0 && compression != 3 {
		return nil, errors.New("unsupported bitmap compression")
	}

	// The masks follow the 40 byte header, which is what BI_BITFIELDS ships for
	// the clipboard. Only the long headers carry an alpha mask, as the fourth
	// field; for the short one the bytes after the three masks are pixel data.
	maskAt := 14 + 40
	count := 3
	if header >= 108 {
		count = 4
	}
	var redMask, greenMask, blueMask, alphaMask uint32
	if compression == 3 && len(data) >= maskAt+count*4 {
		redMask = binary.LittleEndian.Uint32(data[maskAt:])
		greenMask = binary.LittleEndian.Uint32(data[maskAt+4:])
		blueMask = binary.LittleEndian.Uint32(data[maskAt+8:])
		if count == 4 {
			alphaMask = binary.LittleEndian.Uint32(data[maskAt+12:])
		}
	}
	if redMask == 0 || greenMask == 0 || blueMask == 0 {
		if depth == 32 {
			redMask, greenMask, blueMask, alphaMask = 0x00ff0000, 0x0000ff00, 0x000000ff, 0xff000000
		} else {
			redMask, greenMask, blueMask, alphaMask = 0x00ff0000, 0x0000ff00, 0x000000ff, 0
		}
	}

	stride := ((depth*width + 31) / 32) * 4
	if offset+stride*height > len(data) {
		return nil, errors.New("truncated bitmap pixels")
	}

	out := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		row := height - 1 - y
		if topDown {
			row = y
		}
		line := data[offset+row*stride:]
		for x := 0; x < width; x++ {
			var pixel uint32
			if depth == 32 {
				pixel = binary.LittleEndian.Uint32(line[x*4:])
			} else {
				b := line[x*3:]
				pixel = uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
			}
			a := uint8(0xff)
			if alphaMask != 0 {
				a = scaleChannel(pixel, alphaMask)
				// A screenshot often leaves the alpha byte zeroed; treat that as
				// opaque rather than as fully transparent.
				if a == 0 {
					a = 0xff
				}
			}
			out.SetNRGBA(x, y, color.NRGBA{R: scaleChannel(pixel, redMask), G: scaleChannel(pixel, greenMask), B: scaleChannel(pixel, blueMask), A: a})
		}
	}
	return out, nil
}

// scaleChannel pulls one colour channel out of a pixel and stretches it to a
// byte, whatever the mask width is.
func scaleChannel(pixel, mask uint32) uint8 {
	if mask == 0 {
		return 0
	}
	shift := bits.TrailingZeros32(mask)
	value := (pixel & mask) >> uint(shift)
	max := (1 << uint(bits.OnesCount32(mask))) - 1
	if max == 0 {
		return 0
	}
	return uint8(value * 255 / uint32(max))
}
