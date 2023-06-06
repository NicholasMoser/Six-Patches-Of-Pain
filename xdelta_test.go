package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSimpleDelta(t *testing.T) {
	inputPath := "test/SimpleDelta/input.txt"
	tempPath := "test/SimpleDelta/temp.txt"
	fmt.Println("Patching GNT4...")
	input, err := os.Open(inputPath)
	check(err)
	temp, err := os.OpenFile(tempPath, os.O_RDWR|os.O_CREATE, 0644)
	check(err)
	patch, err := os.Open("test/SimpleDelta/patch.xdelta")
	check(err)

	patchWithXdelta(inputPath, input, temp, patch, true)

	if exists(tempPath) && getFileSize(tempPath) > 0 {
		fullPath, err := filepath.Abs(tempPath)
		check(err)
		fmt.Println("Patching complete. Saved to " + fullPath)
	} else {
		t.Fatalf(`patchWithXdelta("") = %q, %v, want "", error`, "lol", err)
	}
}
