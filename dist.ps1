$VERSION = "0.0.6"

pyinstaller --noconfirm --icon=rinnegan.ico six_patches_of_pain.py
tar.exe -acf "dist/Six-Patches-Of-Pain-$VERSION-Mac.zip" README.md six_patches_of_pain.py
tar.exe -acf "dist/Six-Patches-Of-Pain-$VERSION-Linux.zip" README.md six_patches_of_pain.py
New-Item -ItemType Directory -Force -Path dist/six_patches_of_pain/data
Copy-Item -Force -Path data/xdelta3.exe -Destination dist/six_patches_of_pain/data/xdelta3.exe
tar.exe -acf "dist/Six-Patches-Of-Pain-$VERSION-Windows.zip" -C dist six_patches_of_pain
