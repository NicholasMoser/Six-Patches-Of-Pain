//go:generate goversioninfo -icon=rinnegan.ico
package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/cheggaaa/pb/v3"
)

type Iso struct {
	filePath string
	bytes    []byte
	isFile   bool
}

// DATA folder for data files
var DATA = "data"

// CurrentVersion current version to see if a newer version exists
var CurrentVersion = "data/current_version"

// GitRepository git repo to download new releases from
var GitRepositoryFile = "data/git_repository"

// DefaultGitRepository default git repository to download new releases from
var GitRepository = "https://api.github.com/repos/NicholasMoser/SCON4-Releases/releases/latest"

// argGitRepository git repository given as argument to download new releases from
var argGitRepository string

// argSpecificVersion boolean that specifies if you want to select which version to download
var argSpecificVersion bool

// PatchFile the patch file to be downloaded
var PatchFile = "data/patch.xdelta"

// PatchZip the patch zip to be downloaded
var PatchZip = "data/patch.zip"

// VanillaPatch the name of the vanilla xdelta patch in the PatchZip
var VanillaPatch = "vanilla.xdelta"

// GNT4ISOPath path of the GNT4 ISO if it's not in the current directory
var GNT4ISOPath = "data/gnt4_iso_path"

// GNT4ISO default name of the GNT4 iso if the user downloads it
var GNT4ISO = "data/GNT4.iso"

// argISOPath path of the GNT4 ISO given as argument
var argISOPath string

// WindowsExecutableName the name of the Windows executable
var WindowsExecutableName = "Six-Patches-Of-Pain.exe"

// LinuxExecutableName the name of the Mac and Linux executable
var LinuxExecutableName = "Six-Patches-Of-Pain"

// ExecutableName the name of the executable
var ExecutableName string

type githubApiData struct {
	Version string `json:"tag_name"`
	Assets  []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
	}
}

func main() {
	version := "2.0.0"
	fmt.Printf("Starting Six Patches of Pain %s....\n", version)
	fmt.Println()
	argParse()
	verifyIntegrity()
	gnt4Iso := getGNT4ISO()
	var newVersion string
	if argSpecificVersion {
		newVersion = downloadSpecificVersion()
	} else {
		newVersion = downloadNewVersion()
	}
	outputIso := fmt.Sprintf("SCON4-%s.iso", newVersion)
	patchGNT4(gnt4Iso, outputIso)
	setCurrentVersion(newVersion)
	if exists(PatchFile) {
		os.Remove(PatchFile)
	}
	exit(0)
}

// Parse the arguments
func argParse() {
	flag.StringVar(&argGitRepository, "r", "", "Specify git repository to download updates from as 'https://api.github.com/repos/{user}/{repository}/releases'")
	flag.StringVar(&argISOPath, "p", "", "Specify path of the GNT4 ISO")
	flag.BoolVar(&argSpecificVersion, "specific", false, "Select a specific version to download")
	flag.Parse()
}

// Verify the integrity of the auto-updater and required files.
func verifyIntegrity() {
	// Create the data directory if it doesn't already exist
	if !exists(DATA) {
		err := os.Mkdir(DATA, 0755)
		check(err)
	}
	if argGitRepository != "" {
		GitRepository = argGitRepository
	}
	if argISOPath != "" {
		GNT4ISO = argISOPath
	}
	// If git repository is not set, set it to the default release repository. If set, but different from argument, reset if saveConfig arg is set
	if !exists(GitRepositoryFile) {
		d1 := []byte(GitRepository)
		err := ioutil.WriteFile(GitRepositoryFile, d1, 0644)
		check(err)
	}
	if argGitRepository != "" && readFile(GitRepositoryFile) != argGitRepository {
		d1 := []byte(GitRepository)
		err := ioutil.WriteFile(GitRepositoryFile, d1, 0644)
		check(err)
	}
	// If iso path is not set, set it to the default. If set, but different from argument, reset if saveConfig arg is set
	if !exists(GNT4ISOPath) {
		d1 := []byte(GNT4ISO)
		err := ioutil.WriteFile(GNT4ISOPath, d1, 0644)
		check(err)
	}
	if argISOPath != "" && readFile(GNT4ISOPath) != GNT4ISO {
		d1 := []byte(GNT4ISO)
		err := ioutil.WriteFile(GNT4ISOPath, d1, 0644)
		check(err)
	}
	// Delete any existing patch files, since they may be corrupted/old
	if exists(PatchFile) {
		os.Remove(PatchFile)
	}
}

// Retrieves the vanilla GNT4 iso to patch against.
func getGNT4ISO() Iso {
	// First, check if it was drag and dropped onto the executable or provided as an arg
	if len(os.Args) == 2 && !argSpecificVersion {
		var draggedPath = os.Args[1]
		if exists(draggedPath) {
			isGNT4, isoBytes := isGNT4(draggedPath)
			if isGNT4 {
				setGNT4ISOPath(draggedPath)
				if isoBytes != nil {
					return Iso{bytes: isoBytes, isFile: false}
				}
				return Iso{filePath: draggedPath, isFile: true}
			}
			fmt.Println("Provided file is not a vanilla GNT4 ISO: " + draggedPath)
		} else {
			fmt.Println("Provided file does not exist: " + draggedPath)
		}
	}
	// Then look for if it was provided as a named arg
	isoPath := argISOPath
	if exists(isoPath) {
		// If you're using this method, we can hopefully assume it will be a correct vanilla ISO
		return Iso{filePath: isoPath, isFile: true}
	}
	// Then look for the ISO in GNT4_ISO_PATH
	if exists(GNT4ISOPath) {
		isoPath := readFile(GNT4ISOPath)
		if exists(isoPath) {
			isGNT4, isoBytes := isGNT4(isoPath)
			if isGNT4 {
				if isoBytes != nil {
					return Iso{bytes: isoBytes, isFile: false}
				}
				return Iso{filePath: isoPath, isFile: true}
			} else {
				fmt.Println("GNT4_ISO_PATH iso is not a vanilla GNT4 ISO: " + isoPath)
			}
		}
	}
	// If the ISO isn't found from the previous step, look for it recursively in the current directory
	var gnt4Iso Iso
	var gnt4Path string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			isGNT4, isoBytes := isGNT4(path)
			if isGNT4 {
				// Found, stop searching by returning EOF
				if isoBytes != nil {
					gnt4Iso = Iso{bytes: isoBytes, isFile: false}
					gnt4Path = path
				} else {
					gnt4Iso = Iso{filePath: isoPath, isFile: true}
					gnt4Path = path
				}
				return io.EOF
			}
		}
		return nil
	})
	if err != io.EOF {
		check(err)
	}
	if gnt4Iso.bytes != nil || gnt4Iso.filePath != "" {
		setGNT4ISOPath(gnt4Path)
		return gnt4Iso
	}
	// Last resort, query the user for the location of the ISO
	for true {
		fmt.Printf("This updater requires a vanilla GNT4 ISO in order to auto-update.\n")
		fmt.Printf("Please do one of the following:\n")
		fmt.Printf("  - Exit this application and drag and drop your vanilla GNT4 ISO onto %s\n", ExecutableName)
		fmt.Printf("  - Enter the file path to your local copy of a vanilla GNT4 ISO\n")
		fmt.Printf("  - Move a vanilla GNT4 ISO to this folder and restart %s\n", ExecutableName)
		fmt.Printf("  - Enter a link to a download for a vanilla GNT4 ISO\n")
		fmt.Println()
		fmt.Print("Input: ")
		var input string
		fmt.Scanln(&input)
		if exists(input) {
			// Local file
			isGNT4, isoBytes := isGNT4(input)
			if isGNT4 {
				setGNT4ISOPath(input)
				if isoBytes != nil {
					return Iso{bytes: isoBytes, isFile: false}
				}
				return Iso{filePath: input, isFile: true}
			}
			fmt.Printf("\nERROR: %s is not a clean vanilla GNT4 ISO\n\n", input)
		} else {
			// Download from interwebs
			err := download(input, GNT4ISO)
			if err != nil {
				fmt.Printf("Failed to download file with error: %s\n\n", err.Error())
				if exists(GNT4ISO) {
					os.Remove(GNT4ISO)
				}
			} else {
				if exists(GNT4ISO) {
					isGNT4, isoBytes := isGNT4(GNT4ISO)
					if isGNT4 {
						setGNT4ISOPath(GNT4ISO)
						if isoBytes != nil {
							return Iso{bytes: isoBytes, isFile: false}
						}
						return Iso{filePath: GNT4ISO, isFile: true}
					}
					fmt.Printf("\nERROR: Downloaded file was not a vanilla GNT4 ISO.\n\n")
					os.Remove(GNT4ISO)
				}
			}
		}
	}
	return Iso{filePath: "", isFile: true}
}

// Download a new release if it exists and return the version name.
func downloadNewVersion() string {
	// Get the latest release
	repo := readFile(GitRepositoryFile)
	resp, err := http.Get(repo)
	check(err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Printf("Unable to access releases for %s\nStatus code: %s", repo, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	check(err)
	var data githubApiData
	err2 := json.Unmarshal(body, &data)
	check(err2)
	// Stop if the latest release has already been patched locally
	latestVersion := data.Version
	if exists(CurrentVersion) {
		currentVersion := readFile(CurrentVersion)
		if currentVersion == latestVersion {
			fmt.Println("Already on latest SCON4 version: " + latestVersion)
			fmt.Println("If you wish to re-download the latest version,")
			fmt.Println("please delete the file named current_version in the data folder.")
			fail()
		}
	}
	// Download the patch
	if len(data.Assets) == 0 {
		fmt.Println("No assets found in latest release for " + repo)
		fail()
	}
	for i := 0; i < len(data.Assets); i++ {
		asset := data.Assets[i]
		name := asset.Name
		if name == "patch.xdelta" {
			downloadURL := data.Assets[0].DownloadURL
			fmt.Println("\nThere is a new version of SCON4 available: " + latestVersion)
			fmt.Println("Downloading: " + latestVersion)
			download(downloadURL, PatchFile)
			return latestVersion
		} else if name == "patches.zip" {
			downloadURL := data.Assets[0].DownloadURL
			fmt.Println("\nThere is a new version of SCON4 available: " + latestVersion)
			fmt.Println("Downloading: " + latestVersion)
			download(downloadURL, PatchZip)
			unzipPatch()
			os.Remove(PatchZip)
			return latestVersion
		}
	}
	fmt.Println("Unable to find either patch.xdelta or patches.zip")
	fail()
	return ""
}

func unzipPatch() {
	zipListing, err := zip.OpenReader(PatchZip)
	check(err)
	defer zipListing.Close()

	for _, f := range zipListing.File {
		if f.Name != VanillaPatch {
			continue
		}

		// Found vanilla.xdelta
		dstFile, err := os.OpenFile(PatchFile, os.O_RDWR|os.O_CREATE, 0644)
		check(err)
		rc, err := f.Open()
		check(err)
		_, err = io.Copy(dstFile, rc)
		check(err)
		rc.Close()
		return
	}
	fmt.Println("Unable to find vanilla.xdelta")
	fail()
}

// Specify which available version to download
func downloadSpecificVersion() string {
	// Get a specific release
	repo := readFile(GitRepositoryFile)
	resp, err := http.Get(repo)
	check(err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Printf("Unable to access releases for %s\nStatus code: %s", repo, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	check(err)
	var f interface{}
	err2 := json.Unmarshal(body, &f)
	check(err2)
	releases := f.([]interface{})
	if len(releases) == 0 {
		fmt.Println("No releases found at " + repo)
		fail()
	}
	for i := 0; i < len(releases); i++ {
		fmt.Println(i, ": ", releases[i].(map[string]interface{})["name"].(string))
	}
	fmt.Print("Enter the number of the wished release: ")
	var input int
	fmt.Scanln(&input)
	if input >= len(releases) {
		input = len(releases) - 1
	} else if input < 0 {
		input = 0
	}
	specificRelease := releases[input].(map[string]interface{})
	specificVersion := specificRelease["name"].(string)
	// Download the patch
	assets := specificRelease["assets"].([]interface{})
	if len(assets) == 0 {
		fmt.Println("No assets found in latest release for " + repo)
		fail()
	} else if len(assets) > 1 {
		fmt.Println("Too many assets found in latest release for " + repo)
		fail()
	}
	downloadURL := assets[0].(map[string]interface{})["browser_download_url"].(string)
	fmt.Println("Downloading: " + specificVersion)
	download(downloadURL, PatchFile)
	return specificVersion
}

// Patches the given GNT4 ISO to the output SCON4 ISO path using the downloaded patch.
func patchGNT4(gnt4Iso Iso, scon4Iso string) {
	fmt.Println("\nPatching GNT4...")

	if gnt4Iso.isFile {
		// Patch from file input
		input, err := os.Open(gnt4Iso.filePath)
		check(err)
		defer input.Close()
		patchWithXdelta(input, scon4Iso, PatchFile, true)
	} else {
		// Patch from bytes input
		input := bytes.NewReader(gnt4Iso.bytes)
		patchWithXdelta(input, scon4Iso, PatchFile, true)
	}

	if exists(scon4Iso) && getFileSize(scon4Iso) > 0 {
		isoFullPath, err := filepath.Abs(scon4Iso)
		check(err)
		fmt.Println("\nPatching complete. Saved to " + isoFullPath)
	} else {
		fmt.Println("\nFailed to patch ISO, see above messages for more info.")
		exit(1)
	}
}

// Returns whether or not the given file path is vanilla GNT4.
func isGNT4(filePath string) (bool, []byte) {
	if strings.HasSuffix(strings.ToLower(filePath), ".iso") || strings.HasSuffix(strings.ToLower(filePath), ".ciso") {
		f, err := os.Open(filePath)
		check(err)
		data := make([]byte, 6)
		len, err := f.Read(data)
		check(err)
		f.Close()
		expected := []byte("G4NJDA")
		cisoExpected := []byte{0x43, 0x49, 0x53, 0x4F, 0x00, 0x00} // CISO
		if reflect.DeepEqual(expected, data[:len]) || reflect.DeepEqual(cisoExpected, data[:len]) {
			fmt.Println("Validating GNT4 ISO is not modified...")
			hashValue, err := hashFile(filePath)
			check(err)
			// 60aefa3e is the CRC32 hash of both a good ISO dump AND an Nkit ISO somehow
			// Check whether this file is an ISO or an Nkit
			if hashValue == "60aefa3e" {
				if isNkit(filePath) {
					// This is an Nkit ISO, but we currently use a "bad" ISO dump instead.
					// The bad dump is superior as it pads with zeroes instead of random bytes.
					// Confirm the user is okay with modifying their Nkit to be a bad dump.
					fmt.Println("\nConverting NKIT to ISO...")
					isoBytes := convertNkitToIso(filePath)
					return true, isoBytes
				} else {
					// This is a good ISO dump, but we currently use a "bad" dump instead.
					// The bad dump is superior as it pads with zeroes instead of random bytes.
					// Confirm the user is okay with modifying their good dump to be a bad dump.
					fmt.Println("\nConverting good dump ISO to bad dump ISO...")
					isoBytes := patchGoodDump(filePath)
					return true, isoBytes
				}
			} else if hashValue == "0371b18c" {
				// 0371b18c is the CRC32 hash of a ciso file, so we must first convert it to an ISO
				// Confirm the user is okay with modifying their ciso to be an ISO
				fmt.Println("\nConverting CISO to ISO...")
				isoBytes := patchCISO(filePath)
				return true, isoBytes
			}
			return hashValue == "55ee8b1a", nil
		}
	}
	return false, nil
}

// Patches a good dump of vanilla GNT4 to be the expected "bad" dump of GNT4
func patchGoodDump(filePath string) []byte {

	// Read original bytes
	isoBytes, err := os.ReadFile(filePath)
	check(err)

	// First write this weird four byte word to bi2.bin
	copy(isoBytes[0x500:], []byte{0x00, 0x52, 0x02, 0x02})

	var zeroes [4096]byte
	// There are random padding bytes from 0x248104 to 0xC4F8000 (0xC2AFEFC bytes).
	// Replace them with zeroes by looping 49839 times. Then add 3836 extra zeroes.
	for i := 0; i < 49839; i++ {
		offset := 0x248104 + (i * 4096)
		copy(isoBytes[int64(offset):], zeroes[:])
	}
	var moreZeroes [3836]byte
	copy(isoBytes[0xC4F7104:], moreZeroes[:])

	var evenMoreZeroes [11108]byte
	// There are random padding bytes from 0x4553001C - 0x45532B7F (0x2B63 bytes).
	// Just add 11108 zeroes directly.
	copy(isoBytes[0x4553001C:], evenMoreZeroes[:])

	return isoBytes
}

// Patches a CISO of vanilla GNT4 to be the expected "bad" dump of GNT4
func patchCISO(filePath string) []byte {

	// Create byte slice
	isoBytes := make([]byte, 0x57058000)

	// Read sys bytes
	in, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	check(err)
	defer in.Close()
	sys := make([]byte, 0x2480F0)
	_, err = in.ReadAt(sys, 0x8000)
	check(err)

	// Write sys bytes
	copy(isoBytes, sys)

	// Fix sys bytes
	copy(isoBytes[0x200:], make([]byte, 0x14))
	copy(isoBytes[0x500:], []byte{0x00, 0x52, 0x02, 0x02})

	// Copy the rest of the files over
	buf_size := 0x4096
	buf := make([]byte, buf_size)
	i := int64(0x500000)
	offset := int64(0xBFF8000)
	iterations := 0x4AB5D800 / buf_size
	bar := pb.StartNew(iterations)
	bar.Set(pb.Bytes, true)
	bar.Set(pb.SIBytesPrefix, true)
	for {
		num, err := in.ReadAt(buf, i)
		check(err)
		// If near end of where vanilla ISO ends, write the last 0x3CAA bytes and ignore rest of ciso file
		if i+offset == 0x57054356 {
			buf = make([]byte, 0x3CAA)
			_, err := in.ReadAt(buf, i)
			check(err)
			copy(isoBytes[i+offset:], buf)
			break
		}
		if num > 0 {
			copy(isoBytes[i+offset:], buf)
		}
		i += int64(buf_size)
		bar.Increment()
	}
	bar.Finish()

	var evenMoreZeroes [11108]byte
	// There are random padding bytes from 0x4553001C - 0x45532B7F (0x2B63 bytes).
	// Just add 11108 zeroes directly.
	copy(isoBytes[0x4553001C:], evenMoreZeroes[:])

	return isoBytes
}

// Check if an ISO is an nkit ISO
func isNkit(input string) bool {
	in, err := os.OpenFile(input, os.O_RDONLY, 0644)
	check(err)
	defer in.Close()
	bytes := make([]byte, 0x4)
	_, err = in.ReadAt(bytes, 0x200)
	check(err)
	return reflect.DeepEqual(bytes, []byte{0x4E, 0x4B, 0x49, 0x54}) // NKIT
}

// Convert a GNT4 nkit.iso file to a GNT4 iso and return the bytes
func convertNkitToIso(input string) []byte {

	// Create byte slice
	isoBytes := make([]byte, 0x57058000)

	// Read sys bytes
	in, err := os.OpenFile(input, os.O_RDWR, 0644)
	check(err)
	sys := make([]byte, 0x2480F0)
	_, err = in.Read(sys)
	check(err)

	// Write sys bytes
	copy(isoBytes[:], sys)

	// Fix sys bytes
	copy(isoBytes[0x200:], make([]byte, 0x14))
	copy(isoBytes[0x500:], []byte{0x00, 0x52, 0x02, 0x02})
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
			copy(isoBytes[i:], buf)
		}
	}
	copy(isoBytes[0x2480E8:], make([]byte, 0x4))

	// Copy the rest of the files over
	buf_size := 0x4096
	buf := make([]byte, buf_size)
	i := int64(0x250000)
	offset := int64(0xC2A8000)
	iterations := 0x4AB5D800 / buf_size
	bar := pb.StartNew(iterations)
	bar.Set(pb.Bytes, true)
	bar.Set(pb.SIBytesPrefix, true)
	for {
		num, err1 := in.ReadAt(buf, i)
		// Need to write out bytes before EOF check since you can have both EOF and bytes read
		if num > 0 {
			if num != buf_size {
				// Resize buffer to print last bytes minus padding at end of nkit
				buf = buf[:num-0x37C]
			}
			copy(isoBytes[i+offset:], buf)
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
	copy(isoBytes[0x45532B7E:], make([]byte, 0x2))

	return isoBytes
}

// Download to a file path the file at the given url.
func download(url string, filePath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("Unable to download file, status: " + resp.Status)
	}
	bar := pb.Full.Start64(resp.ContentLength)
	bar.Set(pb.Bytes, true)
	bar.Set(pb.SIBytesPrefix, true)
	defer bar.Finish()
	barReader := bar.NewProxyReader(resp.Body)
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, barReader)
	return err
}

//  Set the vanilla GNT4 ISO path to the vanilla GNT4 ISO path file.
func setGNT4ISOPath(filePath string) {
	data := []byte(filePath)
	err := ioutil.WriteFile(GNT4ISOPath, data, 0644)
	check(err)
}

// Set the new version to the current version file.
func setCurrentVersion(version string) {
	data := []byte(version)
	err := ioutil.WriteFile(CurrentVersion, data, 0644)
	check(err)
}

// Retrieves the CRC32 hash of a given file.
func hashFile(filePath string) (string, error) {
	var returnCRC32String string
	fileSize := getFileSize(filePath)
	file, err := os.Open(filePath)
	if err != nil {
		return returnCRC32String, err
	}
	defer file.Close()
	bar := pb.Full.Start64(fileSize)
	bar.Set(pb.Bytes, true)
	bar.Set(pb.SIBytesPrefix, true)
	defer bar.Finish()
	barReader := bar.NewProxyReader(file)
	tablePolynomial := crc32.MakeTable(crc32.IEEE)
	hash := crc32.New(tablePolynomial)
	if _, err := io.Copy(hash, barReader); err != nil {
		return returnCRC32String, err
	}
	hashInBytes := hash.Sum(nil)[:]
	returnCRC32String = hex.EncodeToString(hashInBytes)
	return returnCRC32String, nil
}

// Check if a command is available. Shamelessly borrowed from
// https://siongui.github.io/2018/03/16/go-check-if-command-exists/
func isCommandAvailable(name string) bool {
	cmd := exec.Command("/bin/sh", "-c", "command -v "+name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// Delete the patch file if it exists and exit with exit code 1.
func fail() {
	if exists(PatchFile) {
		os.Remove(PatchFile)
	}
	exit(1)
}

// Read a file to a string
func readFile(filePath string) string {
	content, err := ioutil.ReadFile(filePath)
	check(err)
	return string(content)
}

// Get the file size for a file (assumes the file exists)
func getFileSize(filePath string) int64 {
	fi, err := os.Stat(filePath)
	check(err)
	return fi.Size()
}

// Return whether or not the given path exists.
func exists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
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

// Query user to exit and exit with given code.
func exit(code int) {
	fmt.Println("\nPress enter to exit...")
	var output string
	fmt.Scanln(&output)
	os.Exit(code)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
