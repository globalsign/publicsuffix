package publicsuffix

import (
	"bufio"
	"fmt"
	"os"
)

func Example() {
	// Update to get the latest version of the Public Suffix List from the github repository
	if err := Update(); err != nil {
		panic(err.Error())
	}

	// Check the list version
	var release = Release()
	fmt.Printf("Public Suffix List release: %s", release)

	// Check if a given domain is in the Public Suffix List
	if HasPublicSuffix("example.domain.com") {
		// Do something
	}

	// Get public suffix
	var suffix, icann = PublicSuffix("another.example.domain.com")

	// Write the current Public List to a file
	var file, err = os.Create("list_backup")
	if err != nil {
		panic(err.Error())
	}
	defer file.Close()

	var writer = bufio.NewWriter(file)

	if err := Write(writer); err != nil {
		panic(err.Error())
	}

	// Read and load a list from a file
	file, err = os.Open("list_backup")
	if err != nil {
		panic(err.Error())
	}

	var reader = bufio.NewReader(file)
	if err := Read(reader); err != nil {
		panic(err.Error())
	}

}
