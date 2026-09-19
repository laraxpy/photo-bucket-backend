package service

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"testing"
)

// requireFFmpeg skips the calling test when ffmpeg isn't on PATH, since
// video thumbnail generation shells out to it and CI/dev environments may
// not have it installed.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed, skipping video thumbnail test")
	}
}

// newTestMP4 generates a tiny, valid MP4 (via ffmpeg's lavfi test source)
// for tests that need real decodable video data.
func newTestMP4(t *testing.T) []byte {
	t.Helper()
	requireFFmpeg(t)

	outputFile, err := os.CreateTemp("", "test-video-*.mp4")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)

	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "lavfi",
		"-i", "color=c=blue:s=320x240:d=1",
		"-frames:v", "25",
		"-pix_fmt", "yuv420p",
		outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate test video: %v\n%s", err, out)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read generated test video: %v", err)
	}
	return data
}

func TestExtractVideoFrame(t *testing.T) {
	videoData := newTestMP4(t)

	frame, err := extractVideoFrame(videoData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := image.Decode(bytes.NewReader(frame)); err != nil {
		t.Fatalf("extracted frame is not a valid image: %v", err)
	}
}

func TestGenerateThumbnail_Video(t *testing.T) {
	videoData := newTestMP4(t)

	thumbData, err := generateThumbnail(videoData, "video/mp4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(thumbData))
	if err != nil {
		t.Fatalf("thumbnail is not a valid JPEG: %v", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() > thumbnailMaxDimension || bounds.Dy() > thumbnailMaxDimension {
		t.Errorf("thumbnail size = %dx%d, want within %dx%d", bounds.Dx(), bounds.Dy(), thumbnailMaxDimension, thumbnailMaxDimension)
	}
}
