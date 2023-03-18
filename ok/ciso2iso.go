package main

import (
	"os"

	"github.com/cheggaaa/pb/v3"
)

// Test converting a GNT4 nkit to iso
func main() {
	// Read sys bytes
	in, err := os.OpenFile("../GNT4.ciso", os.O_RDONLY, 0644)
	check(err)
	defer in.Close()
	sys := make([]byte, 0x2480F0)
	_, err = in.ReadAt(sys, 0x8000)
	check(err)

	// Write sys bytes
	out, err := os.Create("../test.iso")
	check(err)
	defer out.Close()
	_, err = out.Write(sys)
	check(err)

	// Fix sys bytes
	_, err = out.WriteAt(make([]byte, 0x14), 0x200)
	check(err)
	_, err = out.WriteAt([]byte{0x00, 0x52, 0x02, 0x02}, 0x500)
	check(err)

	// Copy the rest of the files over
	buf_size := 0x4096
	buf := make([]byte, buf_size)
	i := int64(0x500000)
	offset := int64(0xBFF8000)
	iterations := 0x4AB5D800 / buf_size
	bar := pb.StartNew(iterations)
	for {
		num, err := in.ReadAt(buf, i)
		check(err)
		// If near end of where vanilla ISO ends, write the last 0x3CAA bytes and ignore rest of ciso file
		if i+offset == 0x57054356 {
			buf = make([]byte, 0x3CAA)
			_, err := in.ReadAt(buf, i)
			check(err)
			_, err2 := out.WriteAt(buf, i+offset)
			check(err2)
			break
		}
		if num > 0 {
			_, err2 := out.WriteAt(buf, i+offset)
			check(err2)
		}
		i += int64(buf_size)
		bar.Increment()
	}
	bar.Finish()

	var evenMoreZeroes [11108]byte
	// There are random padding bytes from 0x4553001C - 0x45532B7F (0x2B63 bytes).
	// Just add 11108 zeroes directly.
	_, err = out.WriteAt(evenMoreZeroes[:], 0x4553001C)
	check(err)

}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// Return whether or not an array of int64 contains an int64
func contains(s []int64, val int64) bool {
	for _, v := range s {
		if v == val {
			return true
		}
	}
	return false
}
