package main

import "testing"

func TestGetVideoAspectRatio(t *testing.T) {
	filePath := "./samples/boots-video-horizontal.mp4"
	result, err := getVideoAspectRatio(filePath)
	expected := "16:9"

	if result != expected {
		t.Errorf("getVideoAspectRatio(%s) = %s; want %s", filePath, result, expected)
	}
	if err != nil {
		t.Errorf("getVideoAspectRatio returned an error: %v", err)
	}
}
