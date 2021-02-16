$VERSION = "0.0.6"

# Clean up existing dist directory
Remove-Item -Force -Recurse -Path dist

# Run pyinstaller
pyinstaller --noconfirm --icon=rinnegan.ico six_patches_of_pain.py

# Create Mac and Linux zips
tar.exe -acf "dist/Six-Patches-Of-Pain-$VERSION-Mac.zip" README.md six_patches_of_pain.py
tar.exe -acf "dist/Six-Patches-Of-Pain-$VERSION-Linux.zip" README.md six_patches_of_pain.py

# Create Windows executable
New-Item -ItemType Directory -Force -Path dist/six_patches_of_pain/data
Copy-Item -Force -Path data/xdelta3.exe -Destination dist/six_patches_of_pain/data/xdelta3.exe
Rename-Item -Force -Path dist/six_patches_of_pain -NewName Six-Patches-Of-Pain
tar.exe -acf "dist/Six-Patches-Of-Pain-$VERSION-Windows.zip" -C dist Six-Patches-Of-Pain
