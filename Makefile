windows:	
	go generate 
	go build -o build\\Six-Patches-of-Pain.exe six_patches_of_pain.go 

linux:
	go generate
	go build -o ./build/Six-Patches-of-Pain six_patches_of_pain.go 

mac:
	go generate
	go build -o ./build/Six-Patches-of-Pain six_patches_of_pain.go 

get:
	go get github.com/cheggaaa/pb/v3
	go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo
	go get github.com/Athkore/go-xdelta@46912d43fe2cf5336074311d0c6400327083ee52

clean:
	rm -rf ./build
