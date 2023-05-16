package main

import (
	"encoding/binary"
	"fmt"
	"io"
)

// hdrIndicator
const VCD_DECOMPRESS byte = 0x01
const VCD_CODETABLE byte = 0x02
const VCD_APPHEADER byte = 0x04 // nonstandard?

// winIndicator
const VCD_SOURCE = 0x01
const VCD_TARGET = 0x02
const VCD_ADLER32 = 0x04

/*
	build the default code table (used to encode/decode instructions) specified in RFC 3284
	heavily based on
	https://github.com/vic-alexiev/TelerikAcademy/blob/master/C%23%20Fundamentals%20II/Homework%20Assignments/3.%20Methods/000.%20MiscUtil/Compression/Vcdiff/CodeTable.cs
*/
const VCD_NOOP = 0
const VCD_ADD = 1
const VCD_RUN = 2
const VCD_COPY = 3

/*
	ported from https://github.com/vic-alexiev/TelerikAcademy/tree/master/C%23%20Fundamentals%20II/Homework%20Assignments/3.%20Methods/000.%20MiscUtil/Compression/Vcdiff
	by Victor Alexiev (https://github.com/vic-alexiev)
*/
const VCD_MODE_SELF = 0
const VCD_MODE_HERE = 1

type WindowHeader struct {
	indicator          byte
	sourceLength       uint
	sourcePosition     uint
	hasAdler32         bool
	adler32            uint32
	deltaLength        uint
	targetWindowLength uint
	deltaIndicator     byte
	addRunDataLength   uint
	instructionsLength uint
	addressesLength    uint
}

func patchWithXdelta(scon4Iso string, gnt4Reader io.ReadSeeker, scon4Writer io.Writer, patchReader io.ReadSeeker) {
	fmt.Println("Patching with xdelta...")

	parseHeader(patchReader)

	headerEndOffset := getCurrentOffset(patchReader)

	//calculate target file size
	var newFileSize uint = 0
	for !isEOF(patchReader) {
		winHeader := decodeWindowHeader(patchReader)
		newFileSize += winHeader.targetWindowLength
		length := int64(winHeader.addRunDataLength + winHeader.addressesLength + winHeader.instructionsLength)
		_, err := patchReader.Seek(length, io.SeekCurrent)
		check(err)
	}
	fmt.Printf("New file size %d\n", newFileSize)

	patchReader.Seek(headerEndOffset, io.SeekStart)
}

func parseHeader(reader io.ReadSeeker) {
	_, err := reader.Seek(0x4, io.SeekStart)
	check(err)
	headerIndicator := readU8(reader)

	// VCD_DECOMPRESS
	if headerIndicator&VCD_DECOMPRESS != 0 {
		//has secondary decompressor, read its id
		secondaryDecompressorId := make([]byte, 1)
		_, err := reader.Read(secondaryDecompressorId)
		check(err)

		if secondaryDecompressorId[0] != 0 {
			fmt.Println("not implemented: secondary decompressor")
			exit(1)
		}
	}

	// VCD_CODETABLE
	if headerIndicator&VCD_CODETABLE != 0 {
		codeTableDataLength := read7BitEncodedInt(reader)

		if codeTableDataLength != 0 {
			fmt.Println("not implemented: custom code table")
			exit(1)
		}
	}

	// VCD_APPHEADER
	if headerIndicator&VCD_APPHEADER != 0 {
		// ignore app header data
		appDataLength := int64(read7BitEncodedInt(reader))
		_, err := reader.Seek(appDataLength, io.SeekCurrent)
		check(err)
	}
}

func decodeWindowHeader(reader io.ReadSeeker) WindowHeader {
	windowHeader := WindowHeader{}
	windowHeader.indicator = readU8(reader)
	windowHeader.sourceLength = 0
	windowHeader.sourcePosition = 0
	windowHeader.hasAdler32 = false

	if windowHeader.indicator&VCD_SOURCE != 0 || windowHeader.indicator&VCD_TARGET != 0 {
		windowHeader.sourceLength = read7BitEncodedInt(reader)
		windowHeader.sourcePosition = read7BitEncodedInt(reader)
	}

	windowHeader.deltaLength = read7BitEncodedInt(reader)
	windowHeader.targetWindowLength = read7BitEncodedInt(reader)
	windowHeader.deltaIndicator = readU8(reader) // secondary compression: 1=VCD_DATACOMP,2=VCD_INSTCOMP,4=VCD_ADDRCOMP
	if windowHeader.deltaIndicator != 0 {
		fmt.Printf("unimplemented windowHeader.deltaIndicator: %d\n", windowHeader.deltaIndicator)
		exit(1)
	}

	windowHeader.addRunDataLength = read7BitEncodedInt(reader)
	windowHeader.instructionsLength = read7BitEncodedInt(reader)
	windowHeader.addressesLength = read7BitEncodedInt(reader)

	if (windowHeader.indicator & VCD_ADLER32) == VCD_ADLER32 {
		windowHeader.hasAdler32 = true
		windowHeader.adler32 = readU32(reader)
	}

	return windowHeader
}

func readU8(reader io.ReadSeeker) byte {
	bytes := make([]byte, 1)
	len, err := reader.Read(bytes)
	check(err)
	if len != 1 {
		offset := getCurrentOffset(reader)
		fmt.Printf("Failed to read one byte at offset %d", offset)
		exit(1)
	}
	return bytes[0]
}

func readU32(reader io.ReadSeeker) uint32 {
	bytes := make([]byte, 4)
	len, err := reader.Read(bytes)
	check(err)
	if len != 4 {
		offset := getCurrentOffset(reader)
		fmt.Printf("Failed to read four bytes at offset %d", offset)
		exit(1)
	}
	return binary.BigEndian.Uint32(bytes)
}

func read7BitEncodedInt(reader io.ReadSeeker) uint {
	var num uint = 0
	bits := uint(readU8(reader))
	num = (num << 7) + (bits & 0x7f)
	for bits&0x80 != 0 {
		bits = uint(readU8(reader))
		num = (num << 7) + (bits & 0x7f)
	}
	return num
}

func getCurrentOffset(reader io.ReadSeeker) int64 {
	pos, err := reader.Seek(0, io.SeekCurrent)
	check(err)
	return pos
}

func isEOF(reader io.ReadSeeker) bool {
	len, err := reader.Read(make([]byte, 1))
	if err == io.EOF || len == 0 {
		return true
	}
	if err != nil {
		// Handle other errors
		check(err)
	}
	_, err2 := reader.Seek(-1, io.SeekCurrent)
	check(err2)
	return false
}
