pyinstaller --onefile --icon=rinnegan.ico main.py
Move-Item -Path dist/main.exe -Destination main.exe
tar.exe -a -c -f Windows.zip main.exe data/xdelta3.exe
Move-Item -Path main.exe -Destination dist/main.exe
tar.exe -a -c -f Mac.zip main.py
tar.exe -a -c -f Linux.zip main.py
Remove-Item -Recurse -Force build
Remove-Item -Recurse -Force dist