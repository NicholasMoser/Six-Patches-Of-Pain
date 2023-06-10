package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestTextDelta(t *testing.T) {
	inputPath := "test/TextDelta/input.txt"
	outputPath := "test/TextDelta/output.txt"
	tempPath := "test/TextDelta/temp.txt"
	patchPath := "test/TextDelta/patch.xdelta"

	input, err := os.Open(inputPath)
	check(err)
	defer input.Close()

	os.Remove(tempPath)
	patchWithXdelta(input, tempPath, patchPath, true)

	if exists(tempPath) && getFileSize(tempPath) > 0 {
		if !filesEqual(outputPath, tempPath) {
			t.Fatalf("Files are not equal: %s and %s", outputPath, tempPath)
		}
	} else {
		t.Fatalf("Test output does not exist: %s", tempPath)
	}
	os.Remove(tempPath)
}

func TestImageDelta(t *testing.T) {
	inputPath := "test/ImageDelta/input.jpg"
	outputPath := "test/ImageDelta/output.jpg"
	tempPath := "test/ImageDelta/temp.jpg"
	patchPath := "test/ImageDelta/patch.xdelta"

	input, err := os.Open(inputPath)
	check(err)
	defer input.Close()

	os.Remove(tempPath)
	patchWithXdelta(input, tempPath, patchPath, true)

	if exists(tempPath) && getFileSize(tempPath) > 0 {
		if !filesEqual(outputPath, tempPath) {
			t.Fatalf("Files are not equal: %s and %s", outputPath, tempPath)
		}
	} else {
		t.Fatalf("Test output does not exist: %s", tempPath)
	}
	os.Remove(tempPath)
}

func TestAdler32(t *testing.T) {
	if _adler32([]byte{0, 0}) != 0x00020001 {
		t.Fatal("Failed adler32 comparison")
	}
	if _adler32([]byte{0, 0, 0, 0}) != 0x00040001 {
		t.Fatal("Failed adler32 comparison")
	}
	if _adler32([]byte{1, 2, 3, 4}) != 0x0018000B {
		t.Fatal("Failed adler32 comparison")
	}
	if _adler32([]byte{1, 1, 1, 1, 1, 1, 1, 1}) != 0x002C0009 {
		t.Fatal("Failed adler32 comparison")
	}
	if _adler32([]byte{0xD6, 0xC3, 0xC4, 0x00, 0x04, 0x14, 0x74, 0x65, 0x73, 0x74, 0x32, 0x2E, 0x74, 0x78, 0x74, 0x2F}) != 0x39DB0625 {
		t.Fatal("Failed adler32 comparison")
	}
	if _adler32([]byte{0xFF, 0xFF, 0xFF}) != 0x05FD02FE {
		t.Fatal("Failed adler32 comparison")
	}
	if _adler32([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) != 0xAA6711EF {
		t.Fatal("Failed adler32 comparison")
	}
	if _adler32([]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}) != 0x2D3807F9 {
		t.Fatal("Failed adler32 comparison")
	}
}

func filesEqual(expectedPath string, actualPath string) bool {
	expected, err := ioutil.ReadFile(expectedPath)
	check(err)
	actual, err := ioutil.ReadFile(actualPath)
	check(err)
	return bytes.Equal(expected, actual)
}
