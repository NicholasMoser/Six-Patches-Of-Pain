package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/cheggaaa/pb/v3"
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
	addressStream io.ReadSeeker
	near          []int
	same          []int
}

// Convert an input into and output with a patch. Validate each chunk via checksums if desired.
// The input is a io.ReadSeeker to allow either bytes or a file to be used, since we may
// need to convert bytes in-memory before we call this method.
func patchWithXdelta(input io.ReadSeeker, outputPath string, patchPath string, validate bool) {

	output, err := os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE, 0644)
	check(err)
	defer output.Close() // TODO: https://www.joeshaw.org/dont-defer-close-on-writable-files/

	patch, err := os.Open(patchPath)
	check(err)
	defer patch.Close()

	parseHeader(patch)

	headerEndOffset := getCurrentOffset(patch)

	// Calculate target file size
	newFileSize := 0
	for !isEOF(patch) {
		winHeader := decodeWindowHeader(patch)
		newFileSize += winHeader.targetWindowLength
		length := int64(winHeader.addRunDataLength + winHeader.addressesLength + winHeader.instructionsLength)
		_, err := patch.Seek(length, io.SeekCurrent)
		check(err)
	}

	// Create progress bar
	bar := pb.StartNew(newFileSize)
	bar.Set(pb.Bytes, true)
	bar.Set(pb.SIBytesPrefix, true)

	patch.Seek(int64(headerEndOffset), io.SeekStart)

	cache := getVCDAddressCache(4, 3)
	codeTable := getDefaultCodeTable()
	targetWindowPosition := 0

	// Loop over xdelta windows
	for !isEOF(patch) {
		winHeader := decodeWindowHeader(patch)

		addRunDataStream, err := os.Open(patchPath)
		check(err)
		defer addRunDataStream.Close()
		addRunDataStream.Seek(getCurrentOffset(patch), io.SeekStart)

		instructionsStream, err := os.Open(patchPath)
		check(err)
		defer instructionsStream.Close()
		instructionsStream.Seek(getCurrentOffset(addRunDataStream)+int64(winHeader.addRunDataLength), io.SeekStart)

		addressesStream, err := os.Open(patchPath)
		check(err)
		defer addressesStream.Close()
		addressesStream.Seek(getCurrentOffset(instructionsStream)+int64(winHeader.instructionsLength), io.SeekStart)

		addRunDataIndex := 0
		resetCache(&cache, addressesStream)

		addressesStreamEndOffset := getCurrentOffset(addressesStream)

		// Loop over instructions
		for getCurrentOffset(instructionsStream) < addressesStreamEndOffset {
			//fmt.Printf("Instruction %d / %d\n", getCurrentOffset(instructionsStream), addressesStreamEndOffset)
			instructionIndex := readU8(instructionsStream)

			for i := 0; i < 2; i++ {
				instruction := codeTable[instructionIndex][i]
				size := instruction.size

				if size == 0 && instruction.codeType != VCD_NOOP {
					size = read7BitEncodedInt(instructionsStream)
				}

				if instruction.codeType == VCD_NOOP {
					//fmt.Println("VCD_NOOP")
					continue

				} else if instruction.codeType == VCD_ADD {
					//fmt.Printf("VCD_ADD (%d)\n", size)
					copyToFile2(addRunDataStream, output, addRunDataIndex+targetWindowPosition, size)
					addRunDataIndex += size

				} else if instruction.codeType == VCD_COPY {
					//fmt.Printf("VCD_COPY (%d)\n", size)
					var addr = decodeAddress(&cache, addRunDataIndex+winHeader.sourceLength, instruction.mode)
					var absAddr = 0

					var sourceData io.ReadSeeker
					if addr < winHeader.sourceLength {
						absAddr = winHeader.sourcePosition + addr
						//fmt.Printf("  absAddr = %d\n", absAddr)
						if winHeader.indicator&VCD_SOURCE != 0 {
							//fmt.Println("  VCD_SOURCE")
							sourceData = input
						} else if winHeader.indicator&VCD_TARGET != 0 {
							//fmt.Println("  VCD_TARGET")
							sourceData = output
						}
					} else {
						absAddr = targetWindowPosition + (addr - winHeader.sourceLength)
						//fmt.Printf("  absAddr = %d\n", absAddr)
						sourceData = output
					}

					distance := (targetWindowPosition + addRunDataIndex) - absAddr
					if sourceData == output && size > distance {
						// Slow copy that can handle overlap of reading and writing targets
						// This functionality is usually used to create repeating byte sequences in the target
						repeatLength := size - distance
						totalSize := size
						inputBytes := make([]byte, distance)
						outputBytes := make([]byte, size)
						sourceData.Seek(int64(absAddr), io.SeekStart)
						sourceData.Read(inputBytes) // Read the bytes that we will be repeating
						// Repeatedly iterate over inputBytes and write to outputBytes
						i := 0
						j := 0
						for size > 0 {
							if i == len(inputBytes) {
								i = 0
							}
							outputBytes[j] = inputBytes[i]
							size--
							i++
							j++
						}
						sourceData.Seek(int64(repeatLength), io.SeekCurrent) // Skip repeated bytes we didn't end up reading
						output.WriteAt(outputBytes, int64(targetWindowPosition+addRunDataIndex))
						addRunDataIndex += totalSize
						absAddr += totalSize
					} else {
						// No overlap, fast copy
						buff := make([]byte, size)
						sourceData.Seek(int64(absAddr), io.SeekStart)
						sourceData.Read(buff)
						output.WriteAt(buff, int64(targetWindowPosition+addRunDataIndex))
						addRunDataIndex += size
						absAddr += size
					}

				} else if instruction.codeType == VCD_RUN {
					//fmt.Printf("VCD_RUN (%d)\n", size)
					runByte := readU8(addRunDataStream)
					offset := targetWindowPosition + addRunDataIndex
					//fmt.Printf("  runByte = %d offset = %d\n", runByte, offset)
					buffer := make([]byte, size)
					for i := range buffer {
						buffer[i] = runByte
					}
					output.WriteAt(buffer, int64(offset))

					addRunDataIndex += size
				} else {
					panic("Invalid instruction type found")
				}
			}
		}

		//fmt.Println("Check CRC")
		if validate && winHeader.hasAdler32 {
			current := adler32(output, targetWindowPosition, winHeader.targetWindowLength)
			if winHeader.adler32 != current {
				panic(fmt.Sprintf("Failed CRC check: Got %X but expected %X\n", current, winHeader.adler32))
			}
		}

		patch.Seek(int64(winHeader.addRunDataLength+winHeader.addressesLength+winHeader.instructionsLength), io.SeekCurrent)
		targetWindowPosition += winHeader.targetWindowLength
		//fmt.Printf("Window processed: 0x%X / 0x%X\n", targetWindowPosition, newFileSize)
		bar.SetCurrent(int64(targetWindowPosition))
	}
	bar.Finish()
}

func copyToFile2(stream *os.File, output *os.File, targetOffset int, len int) {
	buffer := make([]byte, len)
	stream.Read(buffer)
	output.WriteAt(buffer, int64(targetOffset))
}

// ADD TEST FOR THIS
/* Adler-32 - https://en.wikipedia.org/wiki/Adler-32#Example_implementation */
const ADLER32_MOD = 0xfff1

func adler32(file *os.File, offset int, len int) uint32 {
	bytes := make([]byte, len)
	n, err := file.ReadAt(bytes, int64(offset))
	check(err)
	if n != len {
		panic(fmt.Sprintf("Failed to read %d bytes but instead read %d", len, n))
	}

	return _adler32(bytes)
}

func _adler32(byteSlice []byte) uint32 {
	a := 1
	b := 0
	len := len(byteSlice)
	for i := 0; i < len; i++ {
		a = (a + int(byteSlice[i])) % ADLER32_MOD
		b = (b + a) % ADLER32_MOD
	}

	return uint32((b << 16) | a) //>>>0;
}

func decodeAddress(cache *AddressCache, here int, mode int) int {
	var address = 0

	if mode == VCD_MODE_SELF {
		address = read7BitEncodedInt(cache.addressStream)
	} else if mode == VCD_MODE_HERE {
		address = here - read7BitEncodedInt(cache.addressStream)
	} else if mode-2 < cache.nearSize { //near cache
		address = cache.near[mode-2] + read7BitEncodedInt(cache.addressStream)
	} else { //same cache
		var m = mode - (2 + cache.nearSize)
		address = cache.same[m*256+int(readU8(cache.addressStream))]
	}

	update(cache, address)
	return address
}

func update(cache *AddressCache, address int) {
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

func resetCache(cache *AddressCache, addressStream io.ReadSeeker) {
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
			panic("Not implemented: secondary decompressor")
		}
	}

	// VCD_CODETABLE
	if headerIndicator&VCD_CODETABLE != 0 {
		codeTableDataLength := read7BitEncodedInt(reader)

		if codeTableDataLength != 0 {
			panic("Not implemented: custom code table")
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
		panic(fmt.Sprintf("unimplemented windowHeader.deltaIndicator: %d\n", windowHeader.deltaIndicator))
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
		panic(fmt.Sprintf("Failed to read one byte at offset %d", offset))
	}
	return bytes[0]
}

func readU32(reader io.ReadSeeker) uint32 {
	bytes := make([]byte, 4)
	len, err := reader.Read(bytes)
	check(err)
	if len != 4 {
		offset := getCurrentOffset(reader)
		panic(fmt.Sprintf("Failed to read four bytes at offset %d", offset))
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
