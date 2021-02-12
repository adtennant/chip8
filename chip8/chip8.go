package chip8

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/rand"
)

const DISPLAY_WIDTH int = 64
const DISPLAY_HEIGHT int = 32

var fontSet = [80]uint8{
	0xF0, 0x90, 0x90, 0x90, 0xF0, //0
	0x20, 0x60, 0x20, 0x20, 0x70, //1
	0xF0, 0x10, 0xF0, 0x80, 0xF0, //2
	0xF0, 0x10, 0xF0, 0x10, 0xF0, //3
	0x90, 0x90, 0xF0, 0x10, 0x10, //4
	0xF0, 0x80, 0xF0, 0x10, 0xF0, //5
	0xF0, 0x80, 0xF0, 0x90, 0xF0, //6
	0xF0, 0x10, 0x20, 0x40, 0x40, //7
	0xF0, 0x90, 0xF0, 0x90, 0xF0, //8
	0xF0, 0x90, 0xF0, 0x10, 0xF0, //9
	0xF0, 0x90, 0xF0, 0x90, 0x90, //A
	0xE0, 0x90, 0xE0, 0x90, 0xE0, //B
	0xF0, 0x80, 0x80, 0x80, 0xF0, //C
	0xE0, 0x90, 0x90, 0x90, 0xE0, //D
	0xF0, 0x80, 0xF0, 0x80, 0xF0, //E
	0xF0, 0x80, 0xF0, 0x80, 0x80, //F
}

type Beeper interface {
	Beep()
}

type Keys interface {
	IsKeyDown(i uint8) bool
	WasKeyReleased(i uint8) bool
}

type Drawer interface {
	Draw(pixels [DISPLAY_HEIGHT][DISPLAY_WIDTH]bool) error
}

type Display struct {
	pixels [DISPLAY_HEIGHT][DISPLAY_WIDTH]bool
	drawer Drawer
}

func (d *Display) Clear() {
	for y := range d.pixels {
		for x := range d.pixels[y] {
			d.pixels[y][x] = false
		}
	}

	d.drawer.Draw(d.pixels)
}

func (d *Display) DrawSprite(x, y uint8, sprite []uint8) uint8 {
	start_x := int(x)
	start_y := int(y)

	vf := uint8(0)

	for row := range sprite {
		if start_y+row >= DISPLAY_HEIGHT {
			break
		}

		for col := 0; col < 8; col++ {
			if start_x+col >= DISPLAY_WIDTH {
				break
			}

			current := d.pixels[start_y+row][start_x+col]
			new := (sprite[row]>>(7-col))&1 != 0

			if current && new {
				d.pixels[start_y+row][start_x+col] = false
				vf = 1
			} else if !current && new {
				d.pixels[start_y+row][start_x+col] = true
			}
		}
	}

	d.drawer.Draw(d.pixels)

	return vf
}

type Instruction uint16

func (i Instruction) opcode() uint16 {
	return uint16(i) & 0xF000
}

func (i Instruction) x() uint8 {
	return uint8((i & 0x0F00) >> 8)
}

func (i Instruction) y() uint8 {
	return uint8((i & 0x00F0) >> 4)
}

func (i Instruction) n() uint8 {
	return uint8(i & 0x000F)
}

func (i Instruction) nn() uint8 {
	return uint8(i & 0x00FF)
}

func (i Instruction) nnn() uint16 {
	return uint16(i) & 0x0FFF
}

func (i Instruction) String() string {
	return fmt.Sprintf("instruction: 0x%04x, opcode: 0x%04x, x: 0x%01x, y: 0x%01x, n: 0x%01x, nn: 0x%02x, nnn: 0x%03x", uint16(i), i.opcode(), i.x(), i.y(), i.n(), i.nn(), i.nnn())
}

type Chip8 struct {
	v  [16]uint8
	i  uint16
	pc uint16

	stack [16]uint16
	sp    uint16

	memory [4096]uint8

	display *Display
	keys    Keys

	delayTimer uint8
	soundTimer uint8

	beeper Beeper
}

func New(keys Keys, beeper Beeper, drawer Drawer) (*Chip8, error) {
	c := &Chip8{
		pc: 0x200,
		display: &Display{
			drawer: drawer,
		},
		keys:   keys,
		beeper: beeper,
	}

	copy(c.memory[:], fontSet[:])

	return c, nil
}

func (c *Chip8) LoadROM(filename string) error {
	b, err := ioutil.ReadFile(filename)

	if err != nil {
		return err
	}

	copy(c.memory[0x200:], b)

	return nil
}

func (c *Chip8) fetch() Instruction {
	instruction := binary.BigEndian.Uint16(c.memory[c.pc : c.pc+2])
	c.pc += 2

	return Instruction(instruction)
}

func unknownInstructionError(pc uint16, instruction *Instruction) error {
	return fmt.Errorf("unknown opcode @ %v: %v", pc, instruction)
}

func (c *Chip8) execute(instruction *Instruction) error {
	switch instruction.opcode() {
	case 0x0000:
		switch instruction.nn() {
		case 0xE0: // clear screen
			c.display.Clear()
		case 0xEE: // return
			c.sp--
			c.pc = c.stack[c.sp]
		default:
			goto unknownInstruction
		}
	case 0x1000: // jump
		c.pc = instruction.nnn()
	case 0x2000: // call
		c.stack[c.sp] = c.pc
		c.sp++
		c.pc = instruction.nnn()
	case 0x3000: // skip
		if c.v[instruction.x()] == instruction.nn() {
			c.pc += 2
		}
	case 0x4000: // skip
		if c.v[instruction.x()] != instruction.nn() {
			c.pc += 2
		}
	case 0x5000: // skip?
		if instruction.n() == 0 {
			if c.v[instruction.x()] == c.v[instruction.y()] {
				c.pc += 2
			}
		} else {
			goto unknownInstruction
		}
	case 0x6000: // set
		c.v[instruction.x()] = instruction.nn()
	case 0x7000: // add
		c.v[instruction.x()] = c.v[instruction.x()] + instruction.nn()
	case 0x8000:
		switch instruction.n() {
		case 0x0: // set
			c.v[instruction.x()] = c.v[instruction.y()]
		case 0x1: // or
			c.v[instruction.x()] |= c.v[instruction.y()]
		case 0x2: // and
			c.v[instruction.x()] &= c.v[instruction.y()]
		case 0x3: // xor
			c.v[instruction.x()] ^= c.v[instruction.y()]
		case 0x4: // add
			result := uint16(c.v[instruction.x()]) + uint16(c.v[instruction.y()])

			if result > 0xFF {
				c.v[0xF] = 1
			} else {
				c.v[0xF] = 0
			}

			c.v[instruction.x()] = uint8(result & 0xFF)
		case 0x5: // sub
			result := uint16(c.v[instruction.x()]) - uint16(c.v[instruction.y()])

			if c.v[instruction.x()] > c.v[instruction.y()] {
				c.v[0xF] = 1
			} else {
				c.v[0xF] = 0
			}

			c.v[instruction.x()] = uint8(result & 0xFF)
		case 0x6: // shift
			c.v[instruction.x()] = c.v[instruction.y()]
			c.v[0xF] = c.v[instruction.x()] & 0x1
			c.v[instruction.x()] = c.v[instruction.x()] >> 1
		case 0x7: // sub
			result := uint16(c.v[instruction.y()]) - uint16(c.v[instruction.x()])

			if c.v[instruction.y()] > c.v[instruction.x()] {
				c.v[0xF] = 1
			} else {
				c.v[0xF] = 0
			}

			c.v[instruction.x()] = uint8(result & 0xFF)
		case 0xE: // shift
			c.v[instruction.x()] = c.v[instruction.y()]
			c.v[0xF] = c.v[instruction.x()] >> 7
			c.v[instruction.x()] = c.v[instruction.x()] << 1
		default:
			goto unknownInstruction
		}
	case 0x9000: // skip
		if instruction.n() == 0 {
			if c.v[instruction.x()] != c.v[instruction.y()] {
				c.pc += 2
			}
		} else {
			goto unknownInstruction
		}
	case 0xA000: // set index
		c.i = instruction.nnn()
	case 0xB000: // jump with offset
		c.pc = uint16(c.v[0]) + instruction.nnn()
	case 0xC000: // random
		c.v[instruction.x()] = uint8(rand.Intn(256)) & instruction.nn()
	case 0xD000: // display
		x := c.v[instruction.x()] % 64
		y := c.v[instruction.y()] % 32
		sprite := c.memory[c.i : c.i+uint16(instruction.n())]

		c.v[0xF] = c.display.DrawSprite(x, y, sprite)
	case 0xE000: // skip if key
		switch instruction.nn() {
		case 0x9E:
			if c.keys.IsKeyDown(c.v[instruction.x()]) {
				c.pc += 2
			}
		case 0xA1:
			if !c.keys.IsKeyDown(c.v[instruction.x()]) {
				c.pc += 2
			}
		default:
			goto unknownInstruction
		}
	case 0xF000:
		switch instruction.nn() {
		// timers
		case 0x07:
			c.v[instruction.x()] = c.delayTimer
		case 0x15:
			c.delayTimer = c.v[instruction.x()]
		case 0x18:
			c.soundTimer = c.v[instruction.x()]
		case 0x1E: // add to index
			c.i += uint16(c.v[instruction.x()])
		case 0x0A: // get key
			released := false

			for i := uint8(0); i < 16; i++ {
				if c.keys.WasKeyReleased(i) {
					c.v[instruction.x()] = uint8(i)
					released = true
					break
				}
			}

			if !released {
				c.pc -= 2
			}
		case 0x29: // font char
			c.i = uint16(c.v[instruction.x()]) * 5
		case 0x33: // decimal conversion
			c.memory[c.i] = c.v[int(instruction.x())] / 100
			c.memory[c.i+1] = (c.v[int(instruction.x())] / 10) % 10
			c.memory[c.i+2] = (c.v[int(instruction.x())] % 100) / 10
		case 0x55: // store
			for x := uint8(0); x < instruction.x()+1; x++ {
				c.memory[c.i+uint16(x)] = c.v[x]
			}
		case 0x65: // load
			for x := uint8(0); x < instruction.x()+1; x++ {
				c.v[x] = c.memory[c.i+uint16(x)]
			}
		default:
			goto unknownInstruction
		}
	default:
		goto unknownInstruction
	}

	return nil

unknownInstruction:
	return unknownInstructionError(c.pc, instruction)
}

func (c *Chip8) Cycle() error {
	instruction := c.fetch()

	err := c.execute(&instruction)
	if err != nil {
		return err
	}

	if c.delayTimer > 0 {
		c.delayTimer--
	}

	if c.soundTimer > 0 {
		c.soundTimer--

		if c.soundTimer == 0 {
			c.beeper.Beep()
		}
	}

	return nil
}
