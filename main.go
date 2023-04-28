package main

import (
	"fmt"
	"log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	coll "github.com/golang-collections/collections/stack"
)

type model struct {
	memory [4096]byte
	pc     int
	index  int16
	regs   [16]byte
	stack  *coll.Stack
	screen [32][64]bool
}

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

func initializeMemory(debug bool) model {
	data, error := os.ReadFile("roms/ibm-logo.ch8")
	if error != nil {
		panic(error)
	}
	new_memory := model{}
	new_memory.pc = 0x200
	new_memory.index = 0
	new_memory.stack = coll.New()
	copy(new_memory.memory[0x200:], data)
	copy(new_memory.memory[0x50:0x9F], FONT)
	if debug {
		for i := 0; i < len(new_memory.memory); i++ {
			fmt.Printf("%04x\t", new_memory.memory[i])
		}
	}
	return new_memory
}

func (m model) Init() tea.Cmd {
	return doTick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, doTick()
	case TickMsg:
		ins := m.memory[m.pc : m.pc+2]
		m.pc += 2
		opCode := (ins[0] >> 4) & 0xff
		x := ins[0] & 0xf
		y := (ins[1] >> 4) & 0xf
		n := ins[1] & 0xf
		nn := ins[1]
		nnn := ((int16(ins[0]) << 8) | int16(ins[1])) & 0xfff
		insArg := instruction{opCode, x, y, n, nn, nnn}
		log.Printf("%x [INS]", ins)
		log.Printf("%x %x %x %x %x %x [XP]", opCode, x, y, n, nn, nnn)
		m = execute(m, insArg)
		return m, doTick()
	default:
		return m, doTick()
	}
}

type TickMsg time.Time

func doTick() tea.Cmd {
	return tea.Tick(1/700*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func execute(m model, ins instruction) model {
	switch ins.opCode {
	case 0:
		if ins.x == 0 && ins.y == 0xe && ins.n == 0 {
			for i := 0; i < 32; i++ {
				for j := 0; j < 64; j++ {
					m.screen[i][j] = false
				}
			}
			break
		} else if ins.x == 0 && ins.y == 0xe && ins.n == 0xe {
			m.pc = m.stack.Pop().(int)
		}
		break
	case 1:
		m.pc = int(ins.nnn)
		break
	case 2:
		m.stack.Push(m.pc)
		m.pc = int(ins.nnn)
		break
	case 3:
		if m.regs[ins.x] == ins.nn {
			m.pc += 2
		}
		break
	case 4:
		if m.regs[ins.x] != ins.nn {
			m.pc += 2
		}
		break
	case 5:
		if m.regs[ins.x] == m.regs[ins.y] {
			m.pc += 2
		}
		break
	case 6:
		m.regs[ins.x] = ins.nn
		break
	case 7:
		m.regs[ins.x] += ins.nn
		break
	case 8:
		alu(&m, ins)
		break
	case 9:
		if m.regs[ins.x] != m.regs[ins.y] {
			m.pc += 2
		}
		break
	case 0xa:
		m.index = int16(ins.nnn) & 0xfff
		break
	case 0xd:
		drawScreen(&m, ins)
		break
	default:
		break
	}
	return m
}

func alu(m *model, ins instruction) {
	switch ins.n {
	case 0:
		m.regs[ins.x] = m.regs[ins.y]
		break
	case 1:
		m.regs[ins.x] |= m.regs[ins.y]
		break
	case 2:
		m.regs[ins.x] &= m.regs[ins.y]
		break
	case 3:
		m.regs[ins.x] ^= m.regs[ins.y]
		break
	case 4:
		if int(m.regs[ins.x])+int(m.regs[ins.y]) > 255 {
			m.regs[0xf] = 1
		} else {
			m.regs[0xf] = 0
		}
		m.regs[ins.x] += m.regs[ins.y]
		break
	case 5:
		if m.regs[ins.x] > m.regs[ins.y] {
			m.regs[0xf] = 1
		} else {
			m.regs[0xf] = 0
		}
		m.regs[ins.x] -= m.regs[ins.y]
		break
	case 6:
		m.regs[ins.x] = m.regs[ins.y]
		m.regs[0xf] = m.regs[ins.y] & 1
		m.regs[ins.x] >>= 1
		break
	case 8:
		if m.regs[ins.y] > m.regs[ins.x] {
			m.regs[0xf] = 1
		} else {
			m.regs[0xf] = 0
		}
		m.regs[ins.x] = m.regs[ins.y] - m.regs[ins.x]
		break
	case 9:
		m.regs[ins.x] = m.regs[ins.y]
		m.regs[0xf] = m.regs[ins.y] & (1 << 7)
		m.regs[ins.x] <<= 1
		break
	}
}

func drawScreen(m *model, ins instruction) {
	x := int16(m.regs[ins.x] % 64)
	y := int16(m.regs[ins.y] % 32)
	var i int16
	var j int16
	turnedOff := false
	for i = 0; i < int16(ins.n)&0xff; i++ {
		for j = 0; j < 8; j++ {
			bit := (m.memory[m.index+i] >> (8 - j - 1)) & 0x1
			if bit == 0 || y+i > 32 || x+j > 64 {
				continue
			}
			if m.screen[y+i][x+j] {
				turnedOff = true
			}
			m.screen[y+i][x+j] = !m.screen[y+i][x+j]
		}
	}
	if turnedOff {
		m.regs[0xf] = 1
	} else {
		m.regs[0xf] = 0
	}
}

func (m model) View() string {
	s := ""
	for _, line := range m.screen {
		for _, col := range line {
			if col {
				s += "█"
			} else {
				s += " "
			}
		}
		s += "\n"
	}
	return s
}

func main() {
	os.Remove("debug.log")
	f, _ := tea.LogToFile("debug.log", "debug")
	defer f.Close()
	p := tea.NewProgram(initializeMemory(false))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
