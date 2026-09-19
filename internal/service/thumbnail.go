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
	thumbnailSmallMaxDimension  = 200
	thumbnailMediumMaxDimension = 800
	thumbnailContentType        = "image/jpeg"
	thumbnailJPEGQuality        = 80
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

// ThumbnailSet holds the two JPEG-encoded thumbnail variants generated for
// an uploaded file: Small for list/grid views, Medium for a full photo
// viewer.
type ThumbnailSet struct {
	Small  []byte
	Medium []byte
}

// generateThumbnails decodes a single source image (directly for images, or
// via a single ffmpeg-extracted frame for videos) and resizes it into both
// thumbnail variants, so we never decode/extract twice. It returns
// errThumbnailUnsupportedType for content types we don't know how to
// generate a thumbnail for, so callers can skip thumbnail generation
// instead of treating it as a real failure.
func generateThumbnails(data []byte, contentType string) (ThumbnailSet, error) {
	var src image.Image
	switch {
	case thumbnailableImageContentTypes[contentType]:
		decoded, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return ThumbnailSet{}, err
		}
		src = decoded
	case thumbnailableVideoContentTypes[contentType]:
		frame, err := extractVideoFrame(data)
		if err != nil {
			return ThumbnailSet{}, err
		}
		decoded, _, err := image.Decode(bytes.NewReader(frame))
		if err != nil {
			return ThumbnailSet{}, err
		}
		src = decoded
	default:
		return ThumbnailSet{}, errThumbnailUnsupportedType
	}

	small, err := resizeToJPEG(src, thumbnailSmallMaxDimension)
	if err != nil {
		return ThumbnailSet{}, err
	}
	medium, err := resizeToJPEG(src, thumbnailMediumMaxDimension)
	if err != nil {
		return ThumbnailSet{}, err
	}
	return ThumbnailSet{Small: small, Medium: medium}, nil
}

func resizeToJPEG(src image.Image, maxDimension int) ([]byte, error) {
	bounds := src.Bounds()
	dstW, dstH := fitWithinSquare(bounds.Dx(), bounds.Dy(), maxDimension)
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
