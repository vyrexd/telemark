package telemark_test

import (
	"fmt"

	"github.com/vyrexd/telemark"
)

func ExampleConvert() {
	fmt.Println(telemark.Convert("This is **bold** and costs $5.50!"))
	// Output: This is *bold* and costs $5\.50\!
}

func ExampleEntities() {
	text, entities := telemark.Entities("hello **world**")
	fmt.Printf("%q\n", text)
	for _, e := range entities {
		fmt.Printf("%s offset=%d length=%d\n", e.Type, e.Offset, e.Length)
	}
	// Output:
	// "hello world"
	// bold offset=6 length=5
}

func ExampleSplit() {
	chunks := telemark.Split("first\n\nsecond\n\nthird", 8)
	for i, c := range chunks {
		fmt.Printf("%d: %q\n", i, c)
	}
	// Output:
	// 0: "first"
	// 1: "second"
	// 2: "third"
}
