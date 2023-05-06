package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
	"unsafe"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eiannone/keyboard"
	beep "github.com/gen2brain/beeep"
	coll "github.com/golang-collections/collections/stack"
	"github.com/rgamba/evtwebsocket"
)

type model struct {
	memory     [4096]byte
	pc         int
	index      int16
	regs       [16]byte
	stack      *wcStack
	screen     [32][64]bool
	delayTimer byte
	soundTimer byte
	shouldQuit bool
}

var (
	timerTicker *time.Ticker
	websocket   evtwebsocket.Conn
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
	new_memory.stack = (*wcStack)(unsafe.Pointer(coll.New()))
	copy(new_memory.memory[0x200:], data)
	copy(new_memory.memory[0x50:0x9F], FONT)
	new_memory.delayTimer = 0
	new_memory.soundTimer = 0
	new_memory.shouldQuit = false
	err := initializeWorkerMemory(new_memory)
	if err != nil {
		panic(err)
	}
	if debug {
		for i := 0; i < len(new_memory.memory); i++ {
			fmt.Printf("%04x\t", new_memory.memory[i])
		}
	}
	return new_memory
}

type wcStack struct{ *coll.Stack }

func (s *wcStack) getStackValues() []int {
	var newStack *wcStack
	*newStack = *s
	var stackValues []int
	for newStack.Len() != 0 {
		stackValues = append(stackValues, newStack.Pop().(int))
	}
	return stackValues
}

func initializeWorkerMemory(m model) error {
	toSend := []map[string]interface{}{{"commandPath": "/setMemory", "memory": m.memory}, {"commandPath": "/setPC", "pc": m.pc}, {"commandPath": "/setIndex", "index": m.index}, {"commandPath": "/resetRegs"}, {"commandPath": "/setStack", "stack": m.stack.getStackValues}, {"commandPath": "/setDelayTimer", "delayTimer": int(m.delayTimer) & 0xffff}, {"commandPath": "/setSoundTimer", "soundTimer": int(m.soundTimer) & 0xffff}, {"commandPath": "/setShouldQuit", "shouldQuit": false}}
	for _, val := range toSend {
		contents, err := json.Marshal(val)
		if err != nil {
			return err
		}
		err = websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		if err != nil {
			return err
		}
	}
	return nil
}

func (m model) Init() tea.Cmd {
	return doTick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.shouldQuit {
		return m, tea.Quit
	}
	contents, _ := json.Marshal(map[string]string{"commandPath": "/getShouldQuit"})
	websocket.Send(evtwebsocket.Msg{Body: []byte(contents), Callback: func(b []byte, c *evtwebsocket.Conn) {
		shouldQuit := false
		contents := json.Unmarshal(b, &shouldQuit)
		if shouldQuit {
			m.shouldQuit = true
		}
	}})
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
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setDelayTimer", "delayTimer": m.delayTimer})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		}
		if m.soundTimer > 0 {
			m.soundTimer -= 1
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setSoundTimer", "soundTimer": m.soundTimer})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
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
	case MemoryMsg:
		m.memory = msg.memory
		return m, doTick()
	case PCMsg:
		m.pc = msg.pc
		return m, doTick()
	case IndexMsg:
		m.index = msg.index
		return m, doTick()
	case RegsMsg:
		m.regs = msg.regs
		return m, doTick()
	case StackMsg:
		m.stack = (*wcStack)(unsafe.Pointer(coll.New()))
		for i := len(msg.stack) - 1; i >= 0; i-- {
			m.stack.Push(msg.stack[i])
		}
		return m, doTick()
	case DelayTimerMsg:
		m.delayTimer = msg.delayTimer
		return m, doTick()
	case SoundTimerMsg:
		m.soundTimer = msg.soundTimer
		return m, doTick()
	case ShouldQuitMsg:
		m.shouldQuit = msg.shouldQuit
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
	MemoryMsg     struct{ memory [4096]byte }
	PCMsg         struct{ pc int }
	IndexMsg      struct{ index int16 }
	RegsMsg       struct{ regs [16]byte }
	StackMsg      struct{ stack []int }
	DelayTimerMsg struct{ delayTimer byte }
	SoundTimerMsg struct{ soundTimer byte }
	ShouldQuitMsg struct{ shouldQuit bool }
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
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setPC", "pc": m.pc})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		}
		break
	case 1:
		m.pc = int(ins.nnn)
		contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setPC", "pc": m.pc})
		websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		break
	case 2:
		m.stack.Push(m.pc)
		m.pc = int(ins.nnn)
		contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setPC", "pc": m.pc})
		websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		contents, _ = json.Marshal(map[string]interface{}{"commandPath": "/setStack", "stack": m.stack.getStackValues()})
		websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		break
	case 3:
		if m.regs[ins.x] == ins.nn {
			m.pc += 2
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setPC", "pc": m.pc})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		}
		break
	case 4:
		if m.regs[ins.x] != ins.nn {
			m.pc += 2
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setPC", "pc": m.pc})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		}
		break
	case 5:
		if m.regs[ins.x] == m.regs[ins.y] {
			m.pc += 2
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setPC", "pc": m.pc})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		}
		break
	case 6:
		m.regs[ins.x] = ins.nn
		contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setReg", "regIndex": ins.x, "value": ins.nn})
		websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		break
	case 7:
		m.regs[ins.x] += ins.nn
		contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setReg", "regIndex": ins.x, "value": ins.nn})
		websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		break
	case 8:
		alu(&m, ins)
		break
	case 9:
		if m.regs[ins.x] != m.regs[ins.y] {
			m.pc += 2
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setPC", "pc": m.pc})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		}
		break
	case 0xa:
		m.index = int16(ins.nnn) & 0xfff
		contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setIndex", "index": m.index})
		websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		break
	case 0xb:
		m.pc = int(ins.nnn) + int(m.regs[0])
		contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setPC", "pc": m.pc})
		websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		break
	case 0xc:
		m.regs[ins.x] = byte(rand.Intn(256)) & ins.nn
		contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setReg", "regIndex": ins.x, "value": m.regs[ins.x]})
		websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
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
				contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setShouldQuit", "shouldQuit": m.shouldQuit})
				websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
			}
			result = string(res)
		case <-time.After(time.Second / 60):
			result = ""
		}
		log.Printf("%s [SKIP]", result)
		if (ins.nn == 0x9e && result == INTMAP[m.regs[ins.x]]) || (ins.nn == 0xa1 && result == "") {
			m.pc += 2
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setPC", "pc": m.pc})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
		}
		break
	case 0xf:
		switch ins.nn {
		case 0x07:
			m.regs[ins.x] = m.delayTimer
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setReg", "regIndex": ins.x, "value": m.regs[ins.x]})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
			break
		case 0x15:
			m.delayTimer = m.regs[ins.x]
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setDelayTimer", "delayTimer": m.regs[ins.x]})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
			break
		case 0x18:
			m.soundTimer = m.regs[ins.x]
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setSoundTimer", "soundTimer": m.regs[ins.x]})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
			break
		case 0x1e:
			m.index += int16(m.regs[ins.x])
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setIndex", "index": m.index})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
			break
		case 0x0a:
			char, _, _ := keyboard.GetSingleKey()
			if char == rune(keyboard.KeyCtrlC) {
				m.shouldQuit = true
				contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setShouldQuit", "shouldQuit": m.shouldQuit})
				websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
			}
			log.Printf("%x [WAITINGFORKEY]", char)
			val, ok := STRMAP[char]
			if ok {
				m.regs[ins.x] = val
				contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setReg", "regIndex": ins.x, "value": m.regs[ins.x]})
				websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
			}
			break
		case 0x29:
			m.index = int16(0x50+5*(m.regs[ins.x]&0xf)) & 0xff
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setIndex", "index": m.index})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
			break
		case 0x33:
			number := m.regs[ins.x]
			m.memory[m.index] = number / 100
			m.memory[m.index+1] = (number % 100) / 10
			m.memory[m.index+2] = number % 10
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/storeMemory", "index": m.index, "toData": m.memory[m.index : m.index+3]})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
			break
		case 0x55:
			var i byte
			for i = 0; i <= ins.x; i++ {
				m.memory[m.index+int16(i)] = m.regs[i]
			}
			contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/storeMemory", "index": m.index, "toData": m.memory[m.index : m.index+int16(i)+1]})
			websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
			break
		case 0x65:
			var i byte
			for i = 0; i <= ins.x; i++ {
				m.regs[i] = m.memory[m.index+int16(i)]
				contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setReg", "regIndex": i, "value": m.regs[i]})
				websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
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
	contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setReg", "regIndex": ins.x, "value": m.regs[ins.x]})
	websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
	if ins.n >= 4 {
		contents, _ := json.Marshal(map[string]interface{}{"commandPath": "/setReg", "regIndex": 0xf, "value": m.regs[0xf]})
		websocket.Send(evtwebsocket.Msg{Body: []byte(contents)})
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
	resp, err := http.Get("http://127.0.0.1:8787/newWoodchip")
	if err != nil {
		log.Println("Could not create worker room...")
		os.Exit(1)
	}
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		log.Println("Could not create worker room...")
		os.Exit(1)
	}
	room := string(body)
	log.Printf("%s [ROOM]", room)
	p := tea.NewProgram(initializeMemory(false))
	timerTicker = time.NewTicker(time.Second / 60)
	go func() {
		for {
			select {
			case <-timerTicker.C:
				p.Send(TimerTickMsg{})
			}
		}
	}()
	websocket = evtwebsocket.Conn{
		OnConnected: func(w *evtwebsocket.Conn) {
			fmt.Println("[WSCONNECTED]")
		},
		OnMessage: func(msg []byte, w *evtwebsocket.Conn) {
			var data map[string]interface{}
			err := json.Unmarshal(msg, &data)
			if err != nil {
				p.Send(tea.Quit)
			}
			mem, ok := data["memory"]
			if ok {
				p.Send(MemoryMsg{mem.([4096]byte)})
			}
			pc, ok := data["pc"]
			if ok {
				p.Send(PCMsg{pc.(int)})
			}
			index, ok := data["index"]
			if ok {
				p.Send(IndexMsg{index.(int16)})
			}
			regs, ok := data["regs"]
			if ok {
				p.Send(RegsMsg{regs.([16]byte)})
			}
			stack, ok := data["stack"]
			if ok {
				p.Send(StackMsg{stack.([]int)})
			}
			delayTimer, ok := data["delayTimer"]
			if ok {
				p.Send(DelayTimerMsg{delayTimer.(byte)})
			}
			soundTimer, ok := data["soundTimer"]
			if ok {
				p.Send(SoundTimerMsg{soundTimer.(byte)})
			}
			shouldQuit, ok := data["shouldQuit"]
			if ok {
				p.Send(ShouldQuitMsg{shouldQuit.(bool)})
			}
		},
		OnError: func(err error) {
			log.Printf("WS error: %s [ERR]", err.Error())
		},
	}
	websocket.Dial(fmt.Sprintf("ws://127.0.0.1:8787/websocket?name=%d", room), "")
	if _, err := p.Run(); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}
