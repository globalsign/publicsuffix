/*
Copyright 2018 GMO GlobalSign Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
