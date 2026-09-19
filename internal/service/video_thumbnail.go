package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	videoFrameTimestamp = "00:00:00.5"
	ffmpegTimeout       = 30 * time.Second
)

// extractVideoFrame shells out to ffmpeg to grab a single JPEG frame from
// the given video data at videoFrameTimestamp. ffmpeg can't read from an
// in-memory buffer for this, so the data is spooled to a temp file first;
// both temp files are cleaned up before returning.
func extractVideoFrame(data []byte) ([]byte, error) {
	inputFile, err := os.CreateTemp("", "video-thumb-src-*.mp4")
	if err != nil {
		return nil, fmt.Errorf("creating temp input file: %w", err)
	}
	defer os.Remove(inputFile.Name())

	if _, err := inputFile.Write(data); err != nil {
		inputFile.Close()
		return nil, fmt.Errorf("writing temp input file: %w", err)
	}
	if err := inputFile.Close(); err != nil {
		return nil, fmt.Errorf("closing temp input file: %w", err)
	}

	outputFile, err := os.CreateTemp("", "video-thumb-out-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("creating temp output file: %w", err)
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)

	ctx, cancel := context.WithTimeout(context.Background(), ffmpegTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-ss", videoFrameTimestamp,
		"-i", inputFile.Name(),
		"-frames:v", "1",
		"-f", "image2",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %w: %s", err, stderr.String())
	}

	return os.ReadFile(outputPath)
}
