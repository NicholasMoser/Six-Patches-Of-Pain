package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cheggaaa/pb/v3"
)

// Test converting a GNT4 nkit to iso
func main() {
	fmt.Println("Converting GNT4 nkit to iso...")

	// Read sys bytes
	in, err := os.OpenFile("../GNT4.nkit.iso", os.O_RDONLY, 0644)
	check(err)
	defer in.Close()
	sys := make([]byte, 0x2480F0)
	_, err = in.Read(sys)
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

	// Fix file system table (fst.bin)
	skip := []int64{0x245250, 0x24525C, 0x24612C, 0x2462B8, 0x246660, 0x246720}
	for i := int64(0x244D28); i < 0x246760; i += 0xC {
		if !contains(skip, i) {
			buf := make([]byte, 0x4)
			_, err := in.ReadAt(buf, i)
			check(err)
			offset := binary.BigEndian.Uint32(buf)
			new_offset := offset + 0xC2A8000
			if i >= 0x245268 {
				new_offset += 0x2B7C
			}
			binary.BigEndian.PutUint32(buf, new_offset)
			_, err = out.WriteAt(buf, i)
			check(err)
		}
	}
	_, err = out.WriteAt(make([]byte, 0x4), 0x2480E8)

	// Copy the rest of the files over
	buf_size := 0x4096
	buf := make([]byte, buf_size)
	i := int64(0x250000)
	offset := int64(0xC2A8000)
	iterations := 0x4AB5D800 / buf_size
	bar := pb.StartNew(iterations)
	for {
		num, err1 := in.ReadAt(buf, i)
		// Need to write out bytes before EOF check since you can have both EOF and bytes read
		if num > 0 {
			if num != buf_size {
				// Resize buffer to print last bytes minus padding at end of nkit
				buf = buf[:num-0x37C]
			}
			_, err2 := out.WriteAt(buf, i+offset)
			check(err2)
		}
		if errors.Is(err1, io.EOF) {
			break
		}
		if i == 0x39282912 {
			// The GNT4 ISO has extra spacing after some files here, so account for that
			offset += 0x2B7C
		}
		i += int64(buf_size)
		bar.Increment()
	}
	bar.Finish()

	// Last little bit of cleanup
	_, err = out.WriteAt(make([]byte, 0x2), 0x45532B7E)
	check(err)

}

func contains(s []int64, val int64) bool {
	for _, v := range s {
		if v == val {
			return true
		}
	}
	return false
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
