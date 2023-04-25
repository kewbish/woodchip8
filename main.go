package main

import (
	"fmt"
	"os"
	"time"

	coll "github.com/golang-collections/collections/stack"
)

var (
	memory [4096]byte
	pc     int
	index  int
	regs   [16]int
	stack  *coll.Stack
)

type instruction struct {
	x   byte
	y   byte
	n   byte
	nn  byte
	nnn int
}

var FONT []byte = []byte{
	// from http://devernay.free.fr/hacks/chip8/C8TECH10.HTM#font
	0xF0, 0x90, 0x90, 0x90, 0xF0, // 0
	0x20, 0x60, 0x20, 0x20, 0x70, // 1
	0xF0, 0x10, 0xF0, 0x80, 0xF0, // 2
	0xF0, 0x10, 0xF0, 0x10, 0xF0, // 3
	0x90, 0x90, 0xF0, 0x10, 0x10, // 4
	0xF0, 0x80, 0xF0, 0x10, 0xF0, // 5
	0xF0, 0x80, 0xF0, 0x90, 0xF0, // 6
	0xF0, 0x10, 0x20, 0x40, 0x40, // 7
	0xF0, 0x90, 0xF0, 0x90, 0xF0, // 8
	0xF0, 0x90, 0xF0, 0x10, 0xF0, // 9
	0xF0, 0x90, 0xF0, 0x90, 0x90, // A
	0xE0, 0x90, 0xE0, 0x90, 0xE0, // B
	0xF0, 0x80, 0x80, 0x80, 0xF0, // C
	0xE0, 0x90, 0x90, 0x90, 0xE0, // D
	0xF0, 0x80, 0xF0, 0x80, 0xF0, // E
	0xF0, 0x80, 0xF0, 0x80, 0x80, // F
}

func initializeMemory(debug bool) {
	data, error := os.ReadFile("chip8-test.ch8")
	if error != nil {
		panic(error)
	}
	copy(memory[0x200:], data)
	copy(memory[0x50:0x9F], FONT)
	if debug {
		for i := 0; i < len(memory); i++ {
			fmt.Printf("%04x\t", memory[i])
		}
	}
	regs = [16]int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	pc = 0x200
	index = 0
	stack = coll.New()
}

func runRom() {
	for true {
		ins := memory[pc : pc+2]
		pc += 2
		x := ins[0] & 0xff
		y := (ins[1] >> 4) & 0xff
		n := ins[1] & 0xff
		nn := ins[1]
		nnn := (int((ins[1]>>4)&0xff) << 12) | int(ins[1])
		insArg := instruction{x, y, n, nn, nnn}
		fmt.Printf("%x %x %x %x %x", insArg.x, insArg.y, insArg.n, insArg.nn, insArg.nnn)
		// short delay, simulate tick
		time.Sleep(1 / 700 * time.Second)
		// break early for temp debugging
		os.Exit(0)
	}
}

func main() {
	initializeMemory(false)
	runRom()
}
