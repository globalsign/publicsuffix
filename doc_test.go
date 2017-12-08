package publicsuffix

import (
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
	fmt.Printf("suffix: %s, icann: %v", suffix, icann)

	// Write the current Public List to a file
	var file, err = os.Create("list_backup")
	if err != nil {
		panic(err.Error())
	}
	defer file.Close()

	if err := Write(file); err != nil {
		panic(err.Error())
	}

	// Read and load a list from a file
	file, err = os.Open("list_backup")
	if err != nil {
		panic(err.Error())
	}
	defer file.Close()

	if err := Read(file); err != nil {
		panic(err.Error())
	}
}
