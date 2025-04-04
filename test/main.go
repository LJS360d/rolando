package main

import "os"

func main() {
	opusFile, _ := os.OpenFile("test.opus", os.O_RDWR|os.O_CREATE, 0644)
	defer opusFile.Close()

}
