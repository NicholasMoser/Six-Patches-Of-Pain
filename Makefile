windows:	
	go generate 
	go build -o build\\Six-Patches-of-Pain.exe six_patches_of_pain.go 

linux:
	go build -o ./build/Six-Patches-of-Pain six_patches_of_pain.go 

mac:
	go build -o ./build/Six-Patches-of-Pain six_patches_of_pain.go 

get:
	go get github.com/cheggaaa/pb/v3
	go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo
	go get github.com/Athkore/go-xdelta

clean:
	rm -rf ./build
