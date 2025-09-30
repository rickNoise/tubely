package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
)

// Possible ouputs: "16:9", "9:16", or "other"
func getVideoAspectRatio(filePath string) (string, error) {
	// Command returns a streams array containing information about the video
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run ffprobe command: %w", err)
	}

	// We care about the width and height fields of the first stream
	type Stream struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	type FFProbeOutput struct {
		Streams []Stream `json:"streams"`
	}

	// Decode ffprobe output from our buffer into JSON
	var ffprobeOutput FFProbeOutput
	decoder := json.NewDecoder(&out)
	if err := decoder.Decode(&ffprobeOutput); err != nil {
		return "", fmt.Errorf("failed to decode JSON: %w", err)
	}

	// Determine output
	if len(ffprobeOutput.Streams) < 1 {
		return "", fmt.Errorf("unable to get aspect ratios from video %s", filePath)
	}
	videoWidth, videoHeight := ffprobeOutput.Streams[0].Width, ffprobeOutput.Streams[0].Height
	aspectRatio := float64(videoWidth) / float64(videoHeight)

	fmt.Printf("parsed, width: %d, height: %d, aspect ratio (rounded): %.2f\n", videoWidth, videoHeight, aspectRatio)
	if math.Abs(aspectRatio-(16.0/9.0)) <= 0.01 {
		return "16:9", nil
	}
	if math.Abs(aspectRatio-(9.0/16.0)) <= 0.01 {
		return "9:16", nil
	}
	return "other", nil
}
