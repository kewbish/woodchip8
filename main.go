package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/MarinX/keylogger"
	tea "github.com/charmbracelet/bubbletea"
	beep "github.com/gen2brain/beeep"
	coll "github.com/golang-collections/collections/stack"
)

type model struct {
	memory        [4096]byte
	pc            int
	index         int16
	regs          [16]byte
	stack         *coll.Stack
	screen        [32][64]bool
	delayTimer    byte
	soundTimer    byte
	waitingForKey byte
}

var (
	timerTicker *time.Ticker
	logger      *keylogger.KeyLogger
	logEvents   chan keylogger.InputEvent
	keys        [16]int
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

var STRMAP map[string]byte = map[string]byte{"1": 0, "2": 1, "3": 2, "4": 3, "Q": 4, "W": 5, "E": 6, "R": 7, "A": 8, "S": 9, "D": 0xa, "F": 0xb, "Z": 0xc, "X": 0xd, "C": 0xe, "V": 0xf}

func initializeMemory(debug bool) model {
	var path string
	if len(os.Args) > 1 {
		path = os.Args[1]
	} else {
		path = "roms/chip8-test.ch8"
	}
	data, error := os.ReadFile(path)
	if error != nil {
		panic(error)
	}
	new_memory := model{}
	new_memory.pc = 0x200
	new_memory.index = 0
	new_memory.stack = coll.New()
	copy(new_memory.memory[0x200:], data)
	copy(new_memory.memory[0x50:0x9F], FONT)
	new_memory.delayTimer = 0
	new_memory.soundTimer = 0
	new_memory.waitingForKey = 0x10
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
			timerTicker.Stop()
			/*for i := 0; i < 16; i++ {
				log.Printf("%x:%x [ENDREG]", i, m.regs[i])
				log.Printf("%x %x %x  [ENDREG]", m.memory[m.index], m.memory[m.index+1], m.memory[m.index+2])
			}
			logger.Close()
			close(logEvents)*/
			return m, tea.Quit
		}
		byteval, ok := STRMAP[msg.String()]
		log.Printf("%x %x %v [KEY]", m.waitingForKey, byteval, keys)
		if ok {
			keys[byteval] = 5
			log.Printf("%v [SETKEY]", keys)
			if m.waitingForKey != 0x10 {
				m.regs[m.waitingForKey] = byteval
				m.waitingForKey = 0x10
			}
		}
		return m, doTick()
	case TimerTickMsg:
		if m.delayTimer > 0 {
			m.delayTimer -= 1
		}
		if m.soundTimer > 0 {
			m.soundTimer -= 1
		}
		return m, doTick()
	/*case KeypressMsg:
	m.keys[msg.key] = msg.direction
	return m, doTick()*/
	case TickMsg:
		if m.waitingForKey != 0x10 {
			return m, doTick()
		}
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
		for i := range keys {
			if keys[i] > 0 {
				keys[i]--
			}
		}
		log.Printf("%v", keys)
		return m, doTick()
	default:
		return m, doTick()
	}
}

type (
	TickMsg      time.Time
	TimerTickMsg struct{}
	KeypressMsg  struct {
		key       byte
		direction bool
	}
)

func doTick() tea.Cmd {
	return tea.Tick(1/700*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func execute(m model, ins instruction) model {
	if m.soundTimer > 0 {
		beep.Beep(440, 1000/60)
	}
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
			res := m.stack.Pop()
			if res != nil {
				m.pc = res.(int)
			}
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
	case 0xb:
		m.pc = int(ins.nnn) + int(m.regs[0])
		break
	case 0xc:
		m.regs[ins.x] = byte(rand.Intn(256)) & ins.nn
		break
	case 0xd:
		drawScreen(&m, ins)
		break
	case 0xe:
		if m.regs[ins.x] < 0 || m.regs[ins.x] > 0xf {
			break
		}
		if (ins.nn == 0x9e && keys[m.regs[ins.x]] != 0) || (ins.nn == 0xa1 && keys[m.regs[ins.x]] == 0) {
			m.pc += 2
		}
		break
	case 0xf:
		switch ins.nn {
		case 0x07:
			m.regs[ins.x] = m.delayTimer
			break
		case 0x15:
			m.delayTimer = m.regs[ins.x]
			break
		case 0x18:
			m.soundTimer = m.regs[ins.x]
			break
		case 0x1e:
			m.index += int16(m.regs[ins.x])
			break
		case 0x0a:
			log.Printf("[WAITING FOR KEY]")
			m.waitingForKey = ins.x
			break
		case 0x29:
			m.index = int16(0x50+5*(m.regs[ins.x]&0xf)) & 0xff
			break
		case 0x33:
			number := m.regs[ins.x]
			m.memory[m.index] = number / 100
			m.memory[m.index+1] = (number % 100) / 10
			m.memory[m.index+2] = number % 10
			break
		case 0x55:
			var i byte
			for i = 0; i <= ins.x; i++ {
				m.memory[m.index+int16(i)] = m.regs[i]
			}
			break
		case 0x65:
			var i byte
			for i = 0; i <= ins.x; i++ {
				m.regs[i] = m.memory[m.index+int16(i)]
			}
			break
		}
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
		oldX := m.regs[ins.x]
		oldY := m.regs[ins.y]
		m.regs[ins.x] += m.regs[ins.y]
		if int(oldX)+int(oldY) > 255 {
			m.regs[0xf] = 1
		} else {
			m.regs[0xf] = 0
		}
		break
	case 5:
		oldX := m.regs[ins.x]
		oldY := m.regs[ins.y]
		m.regs[ins.x] -= m.regs[ins.y]
		if oldX > oldY {
			m.regs[0xf] = 1
		} else {
			m.regs[0xf] = 0
		}
		break
	case 6:
		oldY := m.regs[ins.y]
		m.regs[ins.x] = oldY
		m.regs[ins.x] >>= 1
		m.regs[0xf] = oldY & 1
		break
	case 7:
		oldX := m.regs[ins.x]
		oldY := m.regs[ins.y]
		m.regs[ins.x] = m.regs[ins.y] - m.regs[ins.x]
		if oldY > oldX {
			m.regs[0xf] = 1
		} else {
			m.regs[0xf] = 0
		}
		break
	case 0xe:
		oldY := m.regs[ins.y]
		m.regs[ins.x] = oldY
		m.regs[ins.x] <<= 1
		m.regs[0xf] = (oldY >> 7)
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
				s += "██"
			} else {
				s += "  "
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
	timerTicker = time.NewTicker(time.Second / 60)
	/*logger, _ = keylogger.New("/dev/input/event18")
	logEvents = logger.Read()*/
	p := tea.NewProgram(initializeMemory(false))
	go func() {
		for {
			select {
			case <-timerTicker.C:
				// log.Printf("%d [TICK]", t)
				p.Send(TimerTickMsg{})
			}
		}
	}()
	/*go func() {
		for e := range logEvents {
			switch e.Type {
			case keylogger.EvKey:
				if e.KeyPress() {
					log.Printf("%s [KEYPRESS]", e.KeyString())
					p.Send(KeypressMsg{STRMAP[e.KeyString()], true})
				}
				if e.KeyRelease() {
					log.Printf("%s [KEYRELEASE]", e.KeyString())
					p.Send(KeypressMsg{STRMAP[e.KeyString()], false})
				}
				break
			}
		}
	}()*/
	if _, err := p.Run(); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}
