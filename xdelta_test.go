package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestSimpleDelta(t *testing.T) {
	inputPath := "test/SimpleDelta/input.txt"
	tempPath := "test/SimpleDelta/temp.txt"
	patchPath := "test/SimpleDelta/patch.xdelta"

	patchWithXdelta(inputPath, tempPath, patchPath, true)

	if exists(tempPath) && getFileSize(tempPath) > 0 {
		fullPath, err := filepath.Abs(tempPath)
		check(err)
		fmt.Println("Patching complete. Saved to " + fullPath)
	} else {
		t.Fatal("Oops")
	}
}
