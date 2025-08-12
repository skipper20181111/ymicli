package main

import (
	"fmt"
)

const AppName = "sample-go"

func Greet(name string) string {
	if name == "" {
		name = "world"
	}
	return "Hello, " + name
}

func main() {
	fmt.Println(Greet("world"))
}
