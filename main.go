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
	index  int16
	regs   [16]byte
	stack  *coll.Stack
	screen [32][64]bool
)

type instruction struct {
	opCode byte
	x      byte
	y      byte
	n      byte
	nn     byte
	nnn    int16
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
	pc = 0x200
	index = 0
	stack = coll.New()
}

func runRom() {
	for true {
		ins := memory[pc : pc+2]
		pc += 2
		opCode := (ins[0] >> 4) & 0xff
		x := ins[0] & 0xff
		y := (ins[1] >> 4) & 0xff
		n := ins[1] & 0xff
		nn := ins[1]
		nnn := (int16((ins[1]>>4)&0xff) << 12) | int16(ins[1])
		insArg := instruction{opCode, x, y, n, nn, nnn}
		execute(insArg)
		// short delay, simulate tick
		time.Sleep(1 / 700 * time.Second)
		// break early for temp debugging
		os.Exit(0)
	}
}

func execute(ins instruction) {
	switch ins.opCode {
	case 0:
		if ins.x == 0 && ins.y == 0xe && ins.n == 0 {
			for i := 0; i < 32; i++ {
				for j := 0; j < 64; j++ {
					screen[i][j] = false
				}
			}
			break
		}
		break
	case 1:
		pc = int(ins.nnn)
		break
	case 6:
		regs[ins.x] = ins.nn
		break
	case 7:
		regs[ins.x] += ins.nn
		break
	case 0xa:
		index = int16(ins.nnn)
		break
	default:
		break
	}
}

func main() {
	initializeMemory(false)
	runRom()
}
