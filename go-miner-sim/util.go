package main

import (
	"io"
	"log"
	"os"
)

func mustCp(src, dst string) {
	srcFile, err := os.Open(src)
	check(err)
	defer srcFile.Close()

	destFile, err := os.Create(dst) // creates if file doesn't exist
	check(err)
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile) // check first var for number of bytes copied
	check(err)

	err = destFile.Sync()
	check(err)
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
