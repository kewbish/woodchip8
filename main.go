package main

import (
	"errors"
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"slices"
	"time"

	coll "github.com/golang-collections/collections/stack"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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
	debug      bool
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

type Game struct {
	model        model
	audioContext *audio.Context
	audioPlayer  *audio.Player
}

var (
	REG_TO_KEY_MAP map[byte]ebiten.Key = map[byte]ebiten.Key{1: ebiten.Key1, 2: ebiten.Key2, 3: ebiten.Key3, 0xc: ebiten.Key4, 4: ebiten.KeyQ, 5: ebiten.KeyW, 6: ebiten.KeyE, 0xd: ebiten.KeyR, 7: ebiten.KeyA, 8: ebiten.KeyS, 9: ebiten.KeyD, 0xe: ebiten.KeyF, 0xa: ebiten.KeyZ, 0: ebiten.KeyX, 0xb: ebiten.KeyC, 0xf: ebiten.KeyV}
	KEY_TO_REG_MAP map[ebiten.Key]byte = map[ebiten.Key]byte{ebiten.Key1: 1, ebiten.Key2: 2, ebiten.Key3: 3, ebiten.Key4: 0xc, ebiten.KeyQ: 4, ebiten.KeyW: 5, ebiten.KeyE: 6, ebiten.KeyR: 0xd, ebiten.KeyA: 7, ebiten.KeyS: 8, ebiten.KeyD: 9, ebiten.KeyF: 0xe, ebiten.KeyZ: 0xa, ebiten.KeyX: 0xa, ebiten.KeyC: 0xb, ebiten.KeyV: 0xf}
)

func initializeMemory(debug bool) model {
	var path string
	if len(os.Args) > 1 {
		path = os.Args[1]
	} else {
		path = "roms/keypad.ch8"
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
	new_memory.debug = false
	if debug {
		for i := 0; i < len(new_memory.memory); i++ {
			fmt.Printf("%04x\t", new_memory.memory[i])
		}
		new_memory.debug = true
	}
	return new_memory
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
			res := m.stack.Pop()
			if res != nil {
				m.pc = res.(int)
			}
		}
		break
	case 1:
		m.pc = int(ins.nnn)
		log.Printf("%x [PC]", m.pc)
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
		if (ins.nn == 0x9e && ebiten.IsKeyPressed(REG_TO_KEY_MAP[m.regs[ins.x]])) || (ins.nn == 0xa1 && !ebiten.IsKeyPressed(REG_TO_KEY_MAP[m.regs[ins.x]])) {
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
			keys := make([]ebiten.Key, 16)
			keys = inpututil.AppendJustPressedKeys(keys)
			log.Printf("%v [KEYS, WAIT]", keys)
			if len(keys) == 0 {
				m.pc -= 2 // loop
			} else {
				m.regs[ins.x] = KEY_TO_REG_MAP[keys[0]]
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
	if m.soundTimer > 0 {
		m.soundTimer -= 1
	}
	if m.delayTimer > 0 {
		m.delayTimer -= 1
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

func (g *Game) Update() error {
	keys := make([]ebiten.Key, 16)
	keys = inpututil.AppendPressedKeys(keys)
	if slices.Contains(keys, ebiten.KeyC) && slices.Contains(keys, ebiten.KeyControl) {
		return errors.New("Terminated.")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyA) || !g.model.debug {
		ins := g.model.memory[g.model.pc : g.model.pc+2]
		g.model.pc += 2
		opCode := (ins[0] >> 4) & 0xff
		x := ins[0] & 0xf
		y := (ins[1] >> 4) & 0xf
		n := ins[1] & 0xf
		nn := ins[1]
		nnn := ((int16(ins[0]) << 8) | int16(ins[1])) & 0xfff
		insArg := instruction{opCode, x, y, n, nn, nnn}
		log.Printf("%x [INS] %x [PC]", ins, g.model.pc)
		log.Printf("%x %x %x %x %x %x [XP]", opCode, x, y, n, nn, nnn)
		log.Printf("%x %x %x [REGS]", g.model.regs[0], g.model.regs[1], g.model.index)
		g.model = execute(g.model, insArg)
		playAudio(g)
	}

	return nil
}

// from ebiten sinewave example
type stream struct {
	position  int64
	remaining []byte
}

func (s *stream) Read(buf []byte) (int, error) {
	if len(s.remaining) > 0 {
		n := copy(buf, s.remaining)
		s.remaining = s.remaining[n:]
		return n, nil
	}

	var origBuf []byte
	if len(buf)%4 > 0 {
		origBuf = buf
		buf = make([]byte, len(origBuf)+4-len(origBuf)%4)
	}

	const length = int64(48000 / 440)
	p := s.position / 4
	for i := 0; i < len(buf)/4; i++ {
		const max = 32767
		b := int16(math.Sin(2*math.Pi*float64(p)/float64(length)) * max)
		buf[4*i] = byte(b)
		buf[4*i+1] = byte(b >> 8)
		buf[4*i+2] = byte(b)
		buf[4*i+3] = byte(b >> 8)
		p++
	}

	s.position += int64(len(buf))
	s.position %= length * 4

	if origBuf != nil {
		n := copy(origBuf, buf)
		s.remaining = buf[n:]
		return n, nil
	}
	return len(buf), nil
}

func (s *stream) Close() error {
	return nil
}

func playAudio(g *Game) {
	if g.audioContext == nil {
		g.audioContext = audio.NewContext(48000)
	}
	var err error
	if g.audioPlayer == nil {
		g.audioPlayer, err = g.audioContext.NewPlayer(&stream{})
		g.audioPlayer.SetVolume(0.3)
	}
	if err == nil && g.model.soundTimer > 0 {
		g.audioPlayer.Play()
	} else {
		g.audioPlayer.Close()
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	for i := 0; i < 32; i++ {
		for j := 0; j < 64; j++ {
			if g.model.screen[i][j] {
				screen.Set(j, i, color.White)
			}
		}
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 64, 32
}

func main() {
	os.Remove("debug.log")
	f, _ := os.OpenFile("debug.log", os.O_WRONLY|os.O_CREATE, 0o644)
	defer f.Close()
	log.SetOutput(f)
	timerTicker = time.NewTicker(time.Second / 60)
	model := initializeMemory(false)
	ebiten.SetWindowSize(640, 320)
	ebiten.SetWindowTitle("woodchip8 simulator")
	game := &Game{model: model}
	if err := ebiten.RunGame(game); err != nil {
		log.Print(err)
	}
}
