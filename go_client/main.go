package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eiannone/keyboard"
	beep "github.com/gen2brain/beeep"
	coll "github.com/golang-collections/collections/stack"
)

type model struct {
	memory     [4096]byte
	pc         int
	index      int16
	regs       [16]byte
	stack      *coll.Stack
	screen     [32][64]bool
	delayTimer byte
	soundTimer byte
	shouldQuit bool
}

var timerTicker *time.Ticker

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

var (
	STRMAP map[rune]byte   = map[rune]byte{'1': 0, '2': 1, '3': 2, '4': 3, 'q': 4, 'w': 5, 'e': 6, 'r': 7, 'a': 8, 's': 9, 'd': 0xa, 'f': 0xb, 'z': 0xc, 'x': 0xd, 'c': 0xe, 'v': 0xf}
	INTMAP map[byte]string = map[byte]string{0: "1", 1: "2", 2: "3", 3: "4", 4: "q", 5: "w", 6: "e", 7: "r", 8: "a", 9: "s", 0xa: "d", 0xb: "f", 0xc: "z", 0xd: "x", 0xe: "c", 0xf: "v"}
)

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
	new_memory.shouldQuit = false
	initializeWorkerMemory(new_memory)
	if debug {
		for i := 0; i < len(new_memory.memory); i++ {
			fmt.Printf("%04x\t", new_memory.memory[i])
		}
	}
	return new_memory
}

func initializeWorkerMemory(m model) {
	done := make(chan bool, 8)
	go func() {
		postBody, _ := json.Marshal(map[string][]byte{"memory": m.memory[:]})
		body := bytes.NewBuffer(postBody)
		resp, err := http.Post("http://127.0.0.1:8787/setMemory", "application/json", body)
		if err != nil {
			log.Println("Could not set values to Worker...")
			os.Exit(1)
		}
		defer resp.Body.Close()
		done <- true
	}()
	go func() {
		postBody, _ := json.Marshal(map[string]int{"pc": m.pc})
		body := bytes.NewBuffer(postBody)
		resp, err := http.Post("http://127.0.0.1:8787/setPC", "application/json", body)
		if err != nil {
			log.Println("Could not set values to Worker...")
			os.Exit(1)
		}
		defer resp.Body.Close()
		done <- true
	}()
	go func() {
		postBody, _ := json.Marshal(map[string]int{"index": int(m.index) & 0xffff})
		body := bytes.NewBuffer(postBody)
		resp, err := http.Post("http://127.0.0.1:8787/setIndex", "application/json", body)
		if err != nil {
			log.Println("Could not set values to Worker...")
			os.Exit(1)
		}
		defer resp.Body.Close()
		done <- true
	}()
	go func() {
		postBody, _ := json.Marshal(map[string]string{})
		body := bytes.NewBuffer(postBody)
		resp, err := http.Post("http://127.0.0.1:8787/resetRegs", "application/json", body)
		if err != nil {
			log.Println("Could not set values to Worker...")
			os.Exit(1)
		}
		defer resp.Body.Close()
		done <- true
	}()
	go func() {
		var newStack *coll.Stack
		*newStack = *m.stack
		var values []int
		for newStack.Len() != 0 {
			values = append(values, newStack.Pop().(int))
		}
		postBody, _ := json.Marshal(map[string][]int{"stack": values})
		body := bytes.NewBuffer(postBody)
		resp, err := http.Post("http://127.0.0.1:8787/setStack", "application/json", body)
		if err != nil {
			log.Println("Could not set values to Worker...")
			os.Exit(1)
		}
		defer resp.Body.Close()
		done <- true
	}()
	go func() {
		postBody, _ := json.Marshal(map[string]int{"delayTimer": int(m.index) & 0xffff})
		body := bytes.NewBuffer(postBody)
		resp, err := http.Post("http://127.0.0.1:8787/setDelayTimer", "application/json", body)
		if err != nil {
			log.Println("Could not set values to Worker...")
			os.Exit(1)
		}
		defer resp.Body.Close()
		done <- true
	}()
	go func() {
		postBody, _ := json.Marshal(map[string]int{"soundTimer": int(m.index) & 0xffff})
		body := bytes.NewBuffer(postBody)
		resp, err := http.Post("http://127.0.0.1:8787/setSoundTimer", "application/json", body)
		if err != nil {
			log.Println("Could not set values to Worker...")
			os.Exit(1)
		}
		defer resp.Body.Close()
		done <- true
	}()
	go func() {
		postBody, _ := json.Marshal(map[string]bool{"shouldQuit": false})
		body := bytes.NewBuffer(postBody)
		resp, err := http.Post("http://127.0.0.1:8787/setShouldQuit", "application/json", body)
		if err != nil {
			log.Println("Could not set values to Worker...")
			os.Exit(1)
		}
		defer resp.Body.Close()
	}()
	for i := 0; i < 8; i++ {
		<-done
	}
}

func (m model) Init() tea.Cmd {
	return doTick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.shouldQuit {
		return m, tea.Quit
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			timerTicker.Stop()
			return m, tea.Quit
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
		channel := make(chan rune, 1)
		go func() {
			ch, _, _ := keyboard.GetSingleKey()
			channel <- ch
		}()
		var result string
		select {
		case res := <-channel:
			if res == rune(keyboard.KeyCtrlC) {
				m.shouldQuit = true
			}
			result = string(res)
		case <-time.After(time.Second / 60):
			result = ""
		}
		log.Printf("%s [SKIP]", result)
		if (ins.nn == 0x9e && result == INTMAP[m.regs[ins.x]]) || (ins.nn == 0xa1 && result == "") {
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
			char, _, _ := keyboard.GetSingleKey()
			if char == rune(keyboard.KeyCtrlC) {
				m.shouldQuit = true
			}
			log.Printf("%x [WAITINGFORKEY]", char)
			val, ok := STRMAP[char]
			if ok {
				m.regs[ins.x] = val
			}
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
	p := tea.NewProgram(initializeMemory(false))
	go func() {
		for {
			select {
			case <-timerTicker.C:
				p.Send(TimerTickMsg{})
			}
		}
	}()
	if _, err := p.Run(); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}
