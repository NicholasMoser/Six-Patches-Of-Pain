$VERSION = "0.1.0"

# Recreate dist directory
Remove-Item -Force -Recurse -Path dist -ErrorAction Ignore
New-Item -ItemType Directory -Force -Path dist

# Generate Windows binary
$Env:GOOS = "windows"; $Env:GOARCH = "amd64"
go generate
go build

# Zip Windows binary
tar.exe -acf "dist/Six-Patches-Of-Pain-$VERSION-Windows.zip" Six-Patches-Of-Pain.exe data/xdelta3.exe

# Generate Mac binary
$Env:GOOS = "darwin"; $Env:GOARCH = "amd64"
go build

# Zip Mac binary
tar.exe -acf "dist/Six-Patches-Of-Pain-$VERSION-Mac.zip" Six-Patches-Of-Pain

# Generate Linux binary
$Env:GOOS = "linux"; $Env:GOARCH = "amd64"
go build

# Zip Linux binary
tar.exe -acf "dist/Six-Patches-Of-Pain-$VERSION-Linux.zip" Six-Patches-Of-Pain
