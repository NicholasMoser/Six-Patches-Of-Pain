package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
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
const VCD_NOOP byte = 0
const VCD_ADD byte = 1
const VCD_RUN byte = 2
const VCD_COPY byte = 3

/*
	ported from https://github.com/vic-alexiev/TelerikAcademy/tree/master/C%23%20Fundamentals%20II/Homework%20Assignments/3.%20Methods/000.%20MiscUtil/Compression/Vcdiff
	by Victor Alexiev (https://github.com/vic-alexiev)
*/
const VCD_MODE_SELF = 0
const VCD_MODE_HERE = 1

type WindowHeader struct {
	indicator          byte
	sourceLength       int
	sourcePosition     int
	hasAdler32         bool
	adler32            uint32
	deltaLength        int
	targetWindowLength int
	deltaIndicator     byte
	addRunDataLength   int
	instructionsLength int
	addressesLength    int
}

type Code struct {
	codeType byte
	size     int
	mode     int
}

type AddressCache struct {
	nearSize      int
	sameSize      int
	nextNearSlot  int
	addressStream *Stream
	near          []int
	same          []int
}

type Stream struct {
	fileStream io.ReadSeeker
	offset     int64
}

func patchWithXdelta(scon4Iso string, gnt4 *os.File, scon4 *os.File, patch *os.File, validate bool) {
	fmt.Println("Patching with xdelta...")

	parseHeader(patch)

	headerEndOffset := getCurrentOffset(patch)

	//calculate target file size
	newFileSize := 0
	for !isEOF(patch) {
		winHeader := decodeWindowHeader(patch)
		newFileSize += winHeader.targetWindowLength
		length := int64(winHeader.addRunDataLength + winHeader.addressesLength + winHeader.instructionsLength)
		_, err := patch.Seek(length, io.SeekCurrent)
		check(err)
	}
	fmt.Printf("New file size %d\n", newFileSize)

	patch.Seek(int64(headerEndOffset), io.SeekStart)

	cache := getVCDAddressCache(4, 3)
	codeTable := getDefaultCodeTable()
	targetWindowPosition := 0 //renombrar

	for !isEOF(patch) {
		winHeader := decodeWindowHeader(patch)
		fmt.Printf("Reading header at %d\n", winHeader.sourcePosition)

		addRunDataStream := Stream{fileStream: patch, offset: getCurrentOffset(patch)}
		instructionsStream := Stream{fileStream: patch, offset: addRunDataStream.offset + int64(winHeader.addRunDataLength)}
		addressesStream := Stream{fileStream: patch, offset: instructionsStream.offset + int64(winHeader.instructionsLength)}

		addRunDataIndex := 0
		resetCache(&cache, &addressesStream)

		addressesStreamEndOffset := addressesStream.offset

		fmt.Printf("addressesStreamEndOffset: %d\n", addressesStreamEndOffset)
		for instructionsStream.offset < addressesStreamEndOffset {
			fmt.Printf("%d / %d\n", instructionsStream.offset, addressesStreamEndOffset)
			instructionIndex := readU8FromStream(&instructionsStream)
			// Do we need to reset the offset after calling readU8FromStream????????????????????????????

			for i := 0; i < 2; i++ {
				instruction := codeTable[instructionIndex][i]
				//fmt.Printf("Instruction: %d\n", instruction.codeType)
				size := instruction.size

				if size == 0 && instruction.codeType != VCD_NOOP {
					size = read7BitEncodedIntFromStream(&instructionsStream)
				}

				if instruction.codeType == VCD_NOOP {
					continue

				} else if instruction.codeType == VCD_ADD {
					copyToFile2(addRunDataStream, scon4, addRunDataIndex+targetWindowPosition, size)
					addRunDataIndex += size

				} else if instruction.codeType == VCD_COPY {
					var addr = decodeAddress(cache, addRunDataIndex+winHeader.sourceLength, instruction.mode)
					var absAddr = 0

					// source segment and target segment are treated as if they're concatenated
					var sourceData *os.File
					if addr < winHeader.sourceLength {
						absAddr = winHeader.sourcePosition + addr
						if winHeader.indicator&VCD_SOURCE != 0 {
							sourceData = gnt4
						} else if winHeader.indicator&VCD_TARGET != 0 {
							sourceData = scon4
						}
					} else {
						absAddr = targetWindowPosition + (addr - winHeader.sourceLength)
						sourceData = scon4
					}

					buff := make([]byte, 1)
					for size > 0 {
						size--
						addRunDataIndex++
						absAddr++
						sourceData.ReadAt(buff, int64(absAddr))
						scon4.WriteAt(buff, int64(targetWindowPosition+addRunDataIndex))
					}
				} else if instruction.codeType == VCD_RUN {
					runByte := readU8FromStream(&addRunDataStream)
					offset := targetWindowPosition + addRunDataIndex
					for j := 0; j < size; j++ {
						scon4.WriteAt([]byte{runByte}, int64(offset+j+addRunDataIndex))
					}

					addRunDataIndex += size
				} else {
					fmt.Println("invalid instruction type found")
					exit(1)
				}
			}
		}

		if validate && winHeader.hasAdler32 && (winHeader.adler32 != adler32(scon4, targetWindowPosition, winHeader.targetWindowLength)) {
			fmt.Println("error_crc_output")
			exit(1)
		}

		patch.Seek(int64(winHeader.addRunDataLength+winHeader.addressesLength+winHeader.instructionsLength), io.SeekCurrent)
		targetWindowPosition += winHeader.targetWindowLength
	}
}

func copyToFile2(stream Stream, scon4 *os.File, targetOffset int, len int) {
	// Look at original code and figure out how to use stream here
	offset := stream.offset
	buffer := make([]byte, 1)
	for i := 0; i < len; i++ {
		stream.fileStream.Seek(offset, io.SeekStart)
		stream.fileStream.Read(buffer)
		scon4.WriteAt(buffer, int64(targetOffset+i))
	}
	stream.fileStream.Seek(int64(len), io.SeekCurrent)
}

// ADD TEST FOR THIS
/* Adler-32 - https://en.wikipedia.org/wiki/Adler-32#Example_implementation */
const ADLER32_MOD = 0xfff1

func adler32(scon4 *os.File, offset int, len int) uint32 {
	a := 1
	b := 0
	bytes := make([]byte, len)
	n, err := scon4.Read(bytes)
	check(err)
	if n != len {
		fmt.Printf("Failed to read %d bytes but instead read %d", len, n)
		exit(1)
	}

	for i := 0; i < len; i++ {
		a = (a + int(bytes[i+offset])) % ADLER32_MOD
		b = (b + a) % ADLER32_MOD
	}

	return uint32((b << 16) | a) //>>>0;
}

func decodeAddress(cache AddressCache, here int, mode int) int {
	var address = 0

	if mode == VCD_MODE_SELF {
		address = read7BitEncodedIntFromStream(cache.addressStream)
	} else if mode == VCD_MODE_HERE {
		address = here - read7BitEncodedIntFromStream(cache.addressStream)
	} else if mode-2 < cache.nearSize { //near cache
		address = cache.near[mode-2] + read7BitEncodedIntFromStream(cache.addressStream)
	} else { //same cache
		var m = mode - (2 + cache.nearSize)
		address = cache.same[m*256+int(readU8FromStream(cache.addressStream))]
	}

	update(cache, address)
	return address
}

func update(cache AddressCache, address int) {
	if cache.nearSize > 0 {
		cache.near[cache.nextNearSlot] = address
		cache.nextNearSlot = (cache.nextNearSlot + 1) % cache.nearSize
	}

	if cache.sameSize > 0 {
		cache.same[address%(cache.sameSize*256)] = address
	}
}

func getVCDAddressCache(nearSize int, sameSize int) AddressCache {
	near := make([]int, nearSize)
	same := make([]int, sameSize*256)
	return AddressCache{nearSize: nearSize, sameSize: sameSize, near: near, same: same}
}

func resetCache(cache *AddressCache, addressStream *Stream) {
	cache.nextNearSlot = 0
	cache.addressStream = addressStream
	for i := 0; i < len(cache.near); i++ {
		cache.near[i] = 0
	}
	for i := 0; i < len(cache.same); i++ {
		cache.same[i] = 0
	}
}

func getDefaultCodeTable() [][]Code {
	entries := make([][]Code, 256)
	empty := Code{codeType: VCD_NOOP, size: 0, mode: 0}
	index := 0

	// 0
	entries[index] = make([]Code, 2)
	entries[index][0] = Code{codeType: VCD_RUN, size: 0, mode: 0}
	entries[index][1] = empty
	index++

	// 1,18
	for size := 0; size < 18; size++ {
		entries[index] = make([]Code, 2)
		entries[index][0] = Code{codeType: VCD_ADD, size: size, mode: 0}
		entries[index][1] = empty
		index++
	}

	// 19,162
	for mode := 0; mode < 9; mode++ {
		entries[index] = make([]Code, 2)
		entries[index][0] = Code{codeType: VCD_COPY, size: 0, mode: mode}
		entries[index][1] = empty
		index++
		for size := 4; size < 19; size++ {
			entries[index] = make([]Code, 2)
			entries[index][0] = Code{codeType: VCD_COPY, size: size, mode: mode}
			entries[index][1] = empty
			index++
		}
	}

	// 163,234
	for mode := 0; mode < 6; mode++ {
		for addSize := 1; addSize < 5; addSize++ {
			for copySize := 4; copySize < 7; copySize++ {
				entries[index] = make([]Code, 2)
				entries[index][0] = Code{codeType: VCD_ADD, size: addSize, mode: 0}
				entries[index][1] = Code{codeType: VCD_COPY, size: copySize, mode: mode}
				index++
			}
		}
	}

	// 235,246
	for mode := 6; mode < 9; mode++ {
		for addSize := 1; addSize < 5; addSize++ {
			entries[index] = make([]Code, 2)
			entries[index][0] = Code{codeType: VCD_ADD, size: addSize, mode: 0}
			entries[index][1] = Code{codeType: VCD_COPY, size: 4, mode: mode}
			index++
		}
	}

	// 247,255
	for mode := 0; mode < 9; mode++ {
		entries[index] = make([]Code, 2)
		entries[index][0] = Code{codeType: VCD_COPY, size: 4, mode: mode}
		entries[index][1] = Code{codeType: VCD_ADD, size: 1, mode: 0}
		index++
	}

	return entries
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

func readU8FromStream(stream *Stream) byte {
	stream.fileStream.Seek(int64(stream.offset), io.SeekStart)
	bytes := make([]byte, 1)
	len, err := stream.fileStream.Read(bytes)
	check(err)
	if len != 1 {
		offset := getCurrentOffset(stream.fileStream)
		fmt.Printf("Failed to read one byte at offset %d", offset)
		exit(1)
	}
	stream.offset++
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

func read7BitEncodedInt(reader io.ReadSeeker) int {
	var num int = 0
	bits := int(readU8(reader))
	num = (num << 7) + (bits & 0x7f)
	for bits&0x80 != 0 {
		bits = int(readU8(reader))
		num = (num << 7) + (bits & 0x7f)
	}
	return num
}

func read7BitEncodedIntFromStream(stream *Stream) int {
	var num int = 0
	bits := int(readU8FromStream(stream))
	num = (num << 7) + (bits & 0x7f)
	for bits&0x80 != 0 {
		bits = int(readU8FromStream(stream))
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
