package chunkedreader_test

import (
	"fmt"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/chunkedreader"
)

func ExampleMultiplierList_simple() {
	ml, err := chunkedreader.ParseMultiplierList("2M,10M,50M,200M")
	if err != nil {
		panic(err)
	}
	it := ml.Iter()
	for i := range make([]struct{}, 5) {
		fmt.Printf("%d: %s\n", i, fs.SizeSuffix(it.NextChunkSize()))
	}
	// Output:
	// 0: 2M
	// 1: 10M
	// 2: 50M
	// 3: 200M
	// 4: 200M
}

func ExampleMultiplierList_basic() {
	ml, err := chunkedreader.ParseMultiplierList("128M,1G,x2")
	if err != nil {
		panic(err)
	}
	it := ml.Iter()
	for i := range make([]struct{}, 5) {
		fmt.Printf("%d: %s\n", i, fs.SizeSuffix(it.NextChunkSize()))
	}
	// Output:
	// 0: 128M
	// 1: 1G
	// 2: 2G
	// 3: 4G
	// 4: 8G
}

func ExampleMultiplierList_between() {
	ml, err := chunkedreader.ParseMultiplierList("2k,x3,20k,x2")
	if err != nil {
		panic(err)
	}
	it := ml.Iter()
	for i := range make([]struct{}, 5) {
		fmt.Printf("%d: %s\n", i, fs.SizeSuffix(it.NextChunkSize()))
	}
	// Output:
	// 0: 2k
	// 1: 6k
	// 2: 18k
	// 3: 20k
	// 4: 40k
}
