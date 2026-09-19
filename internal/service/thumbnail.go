package service

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
)

const (
	thumbnailMaxDimension = 400
	thumbnailContentType  = "image/jpeg"
	thumbnailJPEGQuality  = 80
)

var errThumbnailUnsupportedType = errors.New("content type not supported for thumbnails")

var thumbnailableImageContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
}

var thumbnailableVideoContentTypes = map[string]bool{
	"video/mp4": true,
}

// generateThumbnail returns a JPEG-encoded thumbnail that fits within
// thumbnailMaxDimension x thumbnailMaxDimension, preserving aspect ratio.
// For images it decodes the data directly; for videos it extracts a single
// frame via ffmpeg first. It returns errThumbnailUnsupportedType for content
// types we don't know how to generate a thumbnail for, so callers can skip
// thumbnail generation instead of treating it as a real failure.
func generateThumbnail(data []byte, contentType string) ([]byte, error) {
	switch {
	case thumbnailableImageContentTypes[contentType]:
		src, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return resizeToJPEG(src)
	case thumbnailableVideoContentTypes[contentType]:
		frame, err := extractVideoFrame(data)
		if err != nil {
			return nil, err
		}
		src, _, err := image.Decode(bytes.NewReader(frame))
		if err != nil {
			return nil, err
		}
		return resizeToJPEG(src)
	default:
		return nil, errThumbnailUnsupportedType
	}
}

func resizeToJPEG(src image.Image) ([]byte, error) {
	bounds := src.Bounds()
	dstW, dstH := fitWithinSquare(bounds.Dx(), bounds.Dy(), thumbnailMaxDimension)
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: thumbnailJPEGQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fitWithinSquare scales (w, h) down to fit within max x max, preserving
// aspect ratio. Dimensions already within max are left untouched (no upscaling).
func fitWithinSquare(w, h, max int) (int, int) {
	if w <= max && h <= max {
		return w, h
	}

	var newW, newH int
	if w >= h {
		newW = max
		newH = int(float64(h) * float64(max) / float64(w))
	} else {
		newH = max
		newW = int(float64(w) * float64(max) / float64(h))
	}
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}
	return newW, newH
}
