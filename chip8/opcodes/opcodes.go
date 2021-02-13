package opcodes

import "fmt"

type Instruction int

const (
	InstructionUnknown Instruction = iota
	Instruction00E0
	Instruction00EE
	Instruction1NNN
	Instruction2NNN
	Instruction3XNN
	Instruction4XNN
	Instruction5XY0
	Instruction6XNN
	Instruction7XNN
	Instruction8XY0
	Instruction8XY1
	Instruction8XY2
	Instruction8XY3
	Instruction8XY4
	Instruction8XY5
	Instruction8XY6
	Instruction8XY7
	Instruction8XYE
	Instruction9XY0
	InstructionANNN
	InstructionBNNN
	InstructionCXNN
	InstructionDXYN
	InstructionEX9E
	InstructionEXA1
	InstructionFX07
	InstructionFX0A
	InstructionFX15
	InstructionFX18
	InstructionFX1E
	InstructionFX29
	InstructionFX33
	InstructionFX55
	InstructionFX65
)

type Opcode uint16

func (o Opcode) Instruction() Instruction {
	switch uint16(o) & 0xF000 {
	case 0x0000:
		switch o.NN() {
		case 0xE0: // clear screen
			return Instruction00E0
		case 0xEE: // return
			return Instruction00EE
		}
	case 0x1000: // jump
		return Instruction1NNN
	case 0x2000: // call
		return Instruction2NNN
	case 0x3000: // skip
		return Instruction3XNN
	case 0x4000: // skip
		return Instruction4XNN
	case 0x5000: // skip?
		if o.N() == 0 {
			return Instruction5XY0
		}
	case 0x6000: // set
		return Instruction6XNN
	case 0x7000: // add
		return Instruction7XNN
	case 0x8000:
		switch o.N() {
		case 0x0: // set
			return Instruction8XY0
		case 0x1: // or
			return Instruction8XY1
		case 0x2: // and
			return Instruction8XY2
		case 0x3: // xor
			return Instruction8XY3
		case 0x4: // add
			return Instruction8XY4
		case 0x5: // sub
			return Instruction8XY5
		case 0x6: // shift
			return Instruction8XY6
		case 0x7: // sub
			return Instruction8XY7
		case 0xE: // shift
			return Instruction8XYE
		}
	case 0x9000: // skip
		if o.N() == 0 {
			return Instruction9XY0
		}
	case 0xA000: // set index
		return InstructionANNN
	case 0xB000: // jump with offset
		return InstructionBNNN
	case 0xC000: // random
		return InstructionCXNN
	case 0xD000: // display
		return InstructionDXYN
	case 0xE000: // skip if key
		switch o.NN() {
		case 0x9E:
			return InstructionEX9E
		case 0xA1:
			return InstructionEXA1
		}
	case 0xF000:
		switch o.NN() {
		// timers
		case 0x07:
			return InstructionFX07
		case 0x15:
			return InstructionFX15
		case 0x18:
			return InstructionFX18
		case 0x1E: // add to index
			return InstructionFX1E
		case 0x0A: // get key
			return InstructionFX0A
		case 0x29: // font char
			return InstructionFX29
		case 0x33: // decimal conversion
			return InstructionFX33
		case 0x55: // store
			return InstructionFX55
		case 0x65: // load
			return InstructionFX65
		}
	}

	return InstructionUnknown
}

func (o Opcode) X() uint8 {
	return uint8((o & 0x0F00) >> 8)
}

func (o Opcode) Y() uint8 {
	return uint8((o & 0x00F0) >> 4)
}

func (o Opcode) N() uint8 {
	return uint8(o & 0x000F)
}

func (o Opcode) NN() uint8 {
	return uint8(o & 0x00FF)
}

func (o Opcode) NNN() uint16 {
	return uint16(o) & 0x0FFF
}

func (o Opcode) String() string {
	return fmt.Sprintf("opcode: 0x%04x, x: 0x%01x, y: 0x%01x, n: 0x%01x, nn: 0x%02x, nnn: 0x%03x", uint16(o), o.X(), o.Y(), o.N(), o.NN(), o.NNN())
}
