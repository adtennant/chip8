package chip8

import (
	"bytes"
	"chip8/chip8/display"
	"chip8/chip8/opcodes"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
)

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

type Chip8 struct {
	v  [16]uint8
	i  uint16
	pc uint16

	stack [16]uint16
	sp    uint16

	memory [4096]uint8

	delayTimer uint8
	soundTimer uint8

	keys    Keys
	beeper  Beeper
	display *display.Display
}

func New(keys Keys, beeper Beeper, drawer display.Drawer) (*Chip8, error) {
	c := &Chip8{
		pc: 0x200,

		keys:    keys,
		beeper:  beeper,
		display: display.NewDisplay(drawer),
	}

	copy(c.memory[:], fontSet[:])

	return c, nil
}

func (c *Chip8) LoadROM(r io.Reader) error {
	var b bytes.Buffer
	_, err := b.ReadFrom(r)
	if err != nil {
		return fmt.Errorf("failed to load ROM: %v", err)
	}

	copy(c.memory[0x200:], b.Bytes())

	return nil
}

func (c *Chip8) fetchAndDecode() opcodes.Opcode {
	instruction := binary.BigEndian.Uint16(c.memory[c.pc : c.pc+2])
	c.pc += 2

	return opcodes.Opcode(instruction)
}

func (c *Chip8) execute(opcode *opcodes.Opcode) error {
	switch opcode.Instruction() {
	case opcodes.Instruction00E0: // clear screen
		err := c.display.Clear()
		if err != nil {
			return fmt.Errorf("execute 00E0 failed: %v", err)
		}
	case opcodes.Instruction00EE: // return
		c.sp--
		c.pc = c.stack[c.sp]
	case opcodes.Instruction1NNN: // jump
		c.pc = opcode.NNN()
	case opcodes.Instruction2NNN: // call
		c.stack[c.sp] = c.pc
		c.sp++
		c.pc = opcode.NNN()
	case opcodes.Instruction3XNN: // skip
		if c.v[opcode.X()] == opcode.NN() {
			c.pc += 2
		}
	case opcodes.Instruction4XNN: // skip
		if c.v[opcode.X()] != opcode.NN() {
			c.pc += 2
		}
	case opcodes.Instruction5XY0: // skip?
		if c.v[opcode.X()] == c.v[opcode.Y()] {
			c.pc += 2
		}
	case opcodes.Instruction6XNN: // set
		c.v[opcode.X()] = opcode.NN()
	case opcodes.Instruction7XNN: // add
		c.v[opcode.X()] = c.v[opcode.X()] + opcode.NN()
	case opcodes.Instruction8XY0: // set
		c.v[opcode.X()] = c.v[opcode.Y()]
	case opcodes.Instruction8XY1: // or
		c.v[opcode.X()] |= c.v[opcode.Y()]
	case opcodes.Instruction8XY2: // and
		c.v[opcode.X()] &= c.v[opcode.Y()]
	case opcodes.Instruction8XY3: // xor
		c.v[opcode.X()] ^= c.v[opcode.Y()]
	case opcodes.Instruction8XY4: // add
		result := uint16(c.v[opcode.X()]) + uint16(c.v[opcode.Y()])

		if result > 0xFF {
			c.v[0xF] = 1
		} else {
			c.v[0xF] = 0
		}

		c.v[opcode.X()] = uint8(result & 0xFF)
	case opcodes.Instruction8XY5: // sub
		result := uint16(c.v[opcode.X()]) - uint16(c.v[opcode.Y()])

		if c.v[opcode.X()] > c.v[opcode.Y()] {
			c.v[0xF] = 1
		} else {
			c.v[0xF] = 0
		}

		c.v[opcode.X()] = uint8(result & 0xFF)
	case opcodes.Instruction8XY6: // shift
		c.v[opcode.X()] = c.v[opcode.Y()]
		c.v[0xF] = c.v[opcode.X()] & 0x1
		c.v[opcode.X()] = c.v[opcode.X()] >> 1
	case opcodes.Instruction8XY7: // sub
		result := uint16(c.v[opcode.Y()]) - uint16(c.v[opcode.X()])

		if c.v[opcode.Y()] > c.v[opcode.X()] {
			c.v[0xF] = 1
		} else {
			c.v[0xF] = 0
		}

		c.v[opcode.X()] = uint8(result & 0xFF)
	case opcodes.Instruction8XYE: // shift
		c.v[opcode.X()] = c.v[opcode.Y()]
		c.v[0xF] = c.v[opcode.X()] >> 7
		c.v[opcode.X()] = c.v[opcode.X()] << 1
	case opcodes.Instruction9XY0: // skip
		if c.v[opcode.X()] != c.v[opcode.Y()] {
			c.pc += 2
		}
	case opcodes.InstructionANNN: // set index
		c.i = opcode.NNN()
	case opcodes.InstructionBNNN: // jump with offset
		c.pc = uint16(c.v[0]) + opcode.NNN()
	case opcodes.InstructionCXNN: // random
		c.v[opcode.X()] = uint8(rand.Intn(256)) & opcode.NN()
	case opcodes.InstructionDXYN: // display
		x := c.v[opcode.X()] % 64
		y := c.v[opcode.Y()] % 32
		sprite := c.memory[c.i : c.i+uint16(opcode.N())]

		vf, err := c.display.DrawSprite(x, y, sprite)
		if err != nil {
			return fmt.Errorf("execute DXYN failed: %v", err)
		}

		c.v[0xF] = vf
	case opcodes.InstructionEX9E: // skip if key
		if c.keys.IsKeyDown(c.v[opcode.X()]) {
			c.pc += 2
		}
	case opcodes.InstructionEXA1: // skip if not key
		if !c.keys.IsKeyDown(c.v[opcode.X()]) {
			c.pc += 2
		}
	// timers
	case opcodes.InstructionFX07:
		c.v[opcode.X()] = c.delayTimer
	case opcodes.InstructionFX15:
		c.delayTimer = c.v[opcode.X()]
	case opcodes.InstructionFX18:
		c.soundTimer = c.v[opcode.X()]
	case opcodes.InstructionFX1E: // add to index
		c.i += uint16(c.v[opcode.X()])
	case opcodes.InstructionFX0A: // get key
		released := false

		for i := uint8(0); i < 16; i++ {
			if c.keys.WasKeyReleased(i) {
				c.v[opcode.X()] = uint8(i)
				released = true
				break
			}
		}

		if !released {
			c.pc -= 2
		}
	case opcodes.InstructionFX29: // font char
		c.i = uint16(c.v[opcode.X()]) * 5
	case opcodes.InstructionFX33: // decimal conversion
		c.memory[c.i] = c.v[int(opcode.X())] / 100
		c.memory[c.i+1] = (c.v[int(opcode.X())] / 10) % 10
		c.memory[c.i+2] = (c.v[int(opcode.X())] % 100) / 10
	case opcodes.InstructionFX55: // store
		for x := uint8(0); x < opcode.X()+1; x++ {
			c.memory[c.i+uint16(x)] = c.v[x]
		}
	case opcodes.InstructionFX65: // load
		for x := uint8(0); x < opcode.X()+1; x++ {
			c.v[x] = c.memory[c.i+uint16(x)]
		}
	default:
		return fmt.Errorf("unknown opcode @ %v: %v", c.pc, opcode)
	}

	return nil
}

func (c *Chip8) Cycle() error {
	opcode := c.fetchAndDecode()

	if err := c.execute(&opcode); err != nil {
		return fmt.Errorf("failed to execute opcode: %v", err)
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
