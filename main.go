package main

import (
	"fmt"
	"os"
)

var memory [4096]byte

func main() {
	data, error := os.ReadFile("chip8-test.ch8")
	if error != nil {
		panic(error)
	}
	copy(memory[0x200:], data)
	for i := 0; i < len(memory); i++ {
		fmt.Printf("%04x\t", memory[i])
	}
}
