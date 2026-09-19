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

func TestGenerateThumbnails(t *testing.T) {
	t.Run("scales a supported image down to both variants", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 1600, 800))
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			t.Fatalf("failed to encode source JPEG: %v", err)
		}

		thumbs, err := generateThumbnails(buf.Bytes(), "image/jpeg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		cases := []struct {
			name string
			data []byte
			want int
		}{
			{"small", thumbs.Small, thumbnailSmallMaxDimension},
			{"medium", thumbs.Medium, thumbnailMediumMaxDimension},
		}
		for _, tc := range cases {
			decoded, err := jpeg.Decode(bytes.NewReader(tc.data))
			if err != nil {
				t.Fatalf("%s thumbnail is not a valid JPEG: %v", tc.name, err)
			}
			bounds := decoded.Bounds()
			wantH := tc.want / 2
			if bounds.Dx() != tc.want || bounds.Dy() != wantH {
				t.Errorf("%s thumbnail size = %dx%d, want %dx%d", tc.name, bounds.Dx(), bounds.Dy(), tc.want, wantH)
			}
		}
	})

	t.Run("rejects unsupported content types", func(t *testing.T) {
		_, err := generateThumbnails([]byte("whatever"), "application/pdf")
		if err != errThumbnailUnsupportedType {
			t.Errorf("err = %v, want %v", err, errThumbnailUnsupportedType)
		}
	})

	t.Run("returns an error for undecodable image data", func(t *testing.T) {
		_, err := generateThumbnails([]byte("not a real jpeg"), "image/jpeg")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})
}
