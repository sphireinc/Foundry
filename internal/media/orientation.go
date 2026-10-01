package media

import (
	"bytes"
	"encoding/binary"
	"image"
)

// jpegOrientation reads only the EXIF orientation field, with bounded offsets.
// Image encoders drop EXIF, so the pixels must be oriented before resizing.
func jpegOrientation(body []byte) int {
	if len(body) < 2 || body[0] != 0xff || body[1] != 0xd8 {
		return 1
	}
	for offset := 2; offset+4 <= len(body); {
		if body[offset] != 0xff {
			return 1
		}
		marker := body[offset+1]
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		if marker == 0xff {
			offset++
			continue
		}
		if marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			offset += 2
			continue
		}
		size := int(binary.BigEndian.Uint16(body[offset+2 : offset+4]))
		if size < 2 || size > len(body)-offset-2 {
			return 1
		}
		segment := body[offset+4 : offset+2+size]
		if marker == 0xe1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			tiff := segment[6:]
			if len(tiff) < 8 {
				return 1
			}
			var order binary.ByteOrder
			switch string(tiff[:2]) {
			case "II":
				order = binary.LittleEndian
			case "MM":
				order = binary.BigEndian
			default:
				return 1
			}
			if order.Uint16(tiff[2:4]) != 42 {
				return 1
			}
			start := uint64(order.Uint32(tiff[4:8]))
			if start+2 > uint64(len(tiff)) {
				return 1
			}
			count := int(order.Uint16(tiff[start : start+2]))
			for i := 0; i < count; i++ {
				entry := start + 2 + uint64(i)*12 // #nosec G115 -- i is bounded by the uint16 EXIF entry count.
				if entry+12 > uint64(len(tiff)) {
					return 1
				}
				data := tiff[entry : entry+12]
				if order.Uint16(data[:2]) == 0x0112 && order.Uint16(data[2:4]) == 3 && order.Uint32(data[4:8]) == 1 {
					orientation := int(order.Uint16(data[8:10]))
					if orientation >= 1 && orientation <= 8 {
						return orientation
					}
					return 1
				}
			}
		}
		offset += 2 + size
	}
	return 1
}
func orientImage(source image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return source
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	outputWidth, outputHeight := width, height
	if orientation >= 5 {
		outputWidth, outputHeight = height, width
	}
	output := image.NewNRGBA(image.Rect(0, 0, outputWidth, outputHeight))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			targetX, targetY := x, y
			switch orientation {
			case 2:
				targetX = width - 1 - x
			case 3:
				targetX, targetY = width-1-x, height-1-y
			case 4:
				targetY = height - 1 - y
			case 5:
				targetX, targetY = y, x
			case 6:
				targetX, targetY = height-1-y, x
			case 7:
				targetX, targetY = height-1-y, width-1-x
			case 8:
				targetX, targetY = y, width-1-x
			}
			output.Set(targetX, targetY, source.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return output
}

// Animated PNGs must retain all frames; the standard PNG decoder returns one.
func animatedPNG(body []byte) bool {
	if len(body) < 8 || !bytes.Equal(body[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return false
	}
	for offset := 8; offset+12 <= len(body); {
		size := uint64(binary.BigEndian.Uint32(body[offset : offset+4]))
		if size > uint64(len(body)-offset-12) { // #nosec G115 -- loop condition guarantees the remaining length is nonnegative.
			return false
		}
		if string(body[offset+4:offset+8]) == "acTL" {
			return true
		}
		offset += 12 + int(size) // #nosec G115 -- size is bounded by the remaining slice length above.
	}
	return false
}
