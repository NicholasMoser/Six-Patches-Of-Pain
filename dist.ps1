$VERSION = "0.0.7"

# Recreate dist directory
Remove-Item -Force -Recurse -Path dist -ErrorAction Ignore
New-Item -ItemType Directory -Force -Path dist

# Generate binaries
go generate
go build

# Zip binaries
tar.exe -acf "dist/Six-Patches-Of-Pain-$VERSION-Windows.zip" Six-Patches-Of-Pain.exe data/xdelta3.exe
