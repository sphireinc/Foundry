package media

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestEXIFOrientationAndPixels(t *testing.T) {
	for orientation := 1; orientation <= 8; orientation++ {
		tiff := make([]byte, 26)
		copy(tiff, "II")
		binary.LittleEndian.PutUint16(tiff[2:4], 42)
		binary.LittleEndian.PutUint32(tiff[4:8], 8)
		binary.LittleEndian.PutUint16(tiff[8:10], 1)
		binary.LittleEndian.PutUint16(tiff[10:12], 0x0112)
		binary.LittleEndian.PutUint16(tiff[12:14], 3)
		binary.LittleEndian.PutUint32(tiff[14:18], 1)
		// #nosec G115 -- orientation loop is bounded from 1 through 8.
		binary.LittleEndian.PutUint16(tiff[18:20], uint16(orientation))
		body := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, 34}, []byte("Exif\x00\x00")...)
		body = append(body, tiff...)
		if got := jpegOrientation(body); got != orientation {
			t.Fatalf("orientation %d read as %d", orientation, got)
		}
		img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
		img.Set(0, 0, color.NRGBA{R: 255, A: 255})
		transformed := orientImage(img, orientation)
		wantX, wantY := 0, 0
		switch orientation {
		case 2:
			wantX = 2
		case 3:
			wantX, wantY = 2, 1
		case 4:
			wantY = 1
		case 6:
			wantX = 1
		case 7:
			wantX, wantY = 1, 2
		case 8:
			wantY = 2
		}
		red, _, _, _ := transformed.At(wantX, wantY).RGBA()
		if red != 65535 {
			t.Fatalf("orientation %d put pixel in wrong place", orientation)
		}
		if orientation >= 5 && transformed.Bounds().Dx() != 2 {
			t.Fatal("rotated dimensions incorrect")
		}
	}
	for _, body := range [][]byte{nil, {0xff, 0xd8, 0xff, 0xe1, 0xff, 0xff}, {0xff, 0xd8, 0xff, 0xe1, 0, 2}} {
		if jpegOrientation(body) != 1 {
			t.Fatal("malformed EXIF did not fall back safely")
		}
	}
}
