package service

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

func TestFitWithinSquare(t *testing.T) {
	cases := []struct {
		name      string
		w, h, max int
		wantW     int
		wantH     int
	}{
		{"smaller than max is left untouched", 100, 50, 400, 100, 50},
		{"exactly max is left untouched", 400, 400, 400, 400, 400},
		{"wide image scales down by width", 1600, 800, 400, 400, 200},
		{"tall image scales down by height", 800, 1600, 400, 200, 400},
		{"square image scales down evenly", 2000, 2000, 400, 400, 400},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotW, gotH := fitWithinSquare(tc.w, tc.h, tc.max)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Errorf("fitWithinSquare(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tc.w, tc.h, tc.max, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestGenerateThumbnail(t *testing.T) {
	t.Run("scales a supported image down to fit the max dimension", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 1200, 600))
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			t.Fatalf("failed to encode source JPEG: %v", err)
		}

		thumbData, err := generateThumbnail(buf.Bytes(), "image/jpeg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		decoded, err := jpeg.Decode(bytes.NewReader(thumbData))
		if err != nil {
			t.Fatalf("thumbnail is not a valid JPEG: %v", err)
		}
		bounds := decoded.Bounds()
		if bounds.Dx() != thumbnailMaxDimension || bounds.Dy() != thumbnailMaxDimension/2 {
			t.Errorf("thumbnail size = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), thumbnailMaxDimension, thumbnailMaxDimension/2)
		}
	})

	t.Run("rejects unsupported content types", func(t *testing.T) {
		_, err := generateThumbnail([]byte("whatever"), "application/pdf")
		if err != errThumbnailUnsupportedType {
			t.Errorf("err = %v, want %v", err, errThumbnailUnsupportedType)
		}
	})

	t.Run("returns an error for undecodable image data", func(t *testing.T) {
		_, err := generateThumbnail([]byte("not a real jpeg"), "image/jpeg")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})
}
