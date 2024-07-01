package ABhL // pronounced "owl"

import "log"

const RamSize = 1024 * 1024 * 1024 // One megabyte

var Log = log.Printf

type Port interface {
	Read() byte
	Write(byte)
}

type Vm struct {
	a, b, h, l, m, t byte
	pc               uint
	E, F, G          Port
	ram              [RamSize]byte
}

var RegNames = []string{"A", "B", "H", "L", "Mem", "PortE", "PortF", "PortG"}

func (vm *Vm) W() uint {
	return BhlJoin(vm.b, vm.h, vm.l)
}

func (vm *Vm) GetReg(reg byte) byte {
	switch reg {
	case 0:
		return vm.a
	case 1:
		return vm.b
	case 2:
		return vm.h
	case 3:
		return vm.l
	case 4:
		return vm.m // Note during IPL, this is not vm.ram[vm.W()]
	case 5:
		if vm.E == nil {
			panic("No device to read at port E")
		}
		return vm.E.Read()
	case 6:
		if vm.F == nil {
			panic("No device to read at port F")
		}
		return vm.F.Read()
	case 7:
		if vm.G == nil {
			panic("No device to read at port G")
		}
		return vm.G.Read()
	default:
		panic("bad reg num")
	}
}

func (vm *Vm) PutReg(reg byte, val byte) {
	switch reg {
	case 0:
		vm.a = val
	case 1:
		vm.b = val
	case 2:
		vm.h = val
	case 3:
		vm.l = val
	case 4:
		vm.ram[vm.W()] = val
	case 5:
		if vm.E == nil {
			panic("No device to write at port E")
		}
		vm.E.Write(val)
	case 6:
		if vm.F == nil {
			panic("No device to write at port F")
		}
		vm.F.Write(val)
	case 7:
		if vm.G == nil {
			panic("No device to write at port G")
		}
		vm.G.Write(val)
	default:
		panic("bad reg num")
	}
}

func (vm *Vm) Steps(n int) bool {
	for i := 0; i < n; i++ {
		vm.t = vm.ram[vm.pc]
		vm.m = vm.ram[vm.W()]
		Log("Step %x. pc=%06x t=%02x", i, vm.pc, vm.t)
		vm.pc++
		ok := vm.Execute()
		Log(".....%x: pc=%06x a=%02x w=%06x mem=% 3x ...", i, vm.pc, vm.a, vm.W(), vm.ram[:16])
		if !ok {
			return false // stopped short
		}
	}
	return true // all steps succeeded
}

// IPL for Initial Program Load.
// Pairs of bytes from vec are injected into t and m at each step.
func (vm *Vm) IPL(vec []byte) {
	n := len(vec)
	for len(vec) != 0 {
		i := n - len(vec)
		vm.t = vec[0]
		vm.m = vec[1]
		Log("IPL %x. pc=%06x t=%02x", i, vm.pc, vm.t)
		vec = vec[2:] // In IPL mode, always consume a fetch and an execute value.
		ok := vm.Execute()
		Log(".....%x: pc=%06x a=%02x w=%06x mem=% 3x ...", i, vm.pc, vm.a, vm.W(), vm.ram[:16])
		if !ok {
			panic("IPL stopped short")
		}
	}
}

func (vm *Vm) Execute() bool {
	t := vm.t
	switch t >> 6 {
	case 0x00:
		r := 3 & t
		switch t & 0x3C {
		case 0x00: // undefined
			return false
		case 0x04: // SETr
			Log("    SET%s immediate $%x", RegNames[r], vm.m)
			vm.PutReg(t&3, vm.m)
			vm.pc++
		case 0x08: // Inc/Dec
			switch 3 & t {
			case 0:
				vm.a++
				Log("    INCA becomes %x", vm.a)
			case 1:
				vm.b, vm.h, vm.l = BhlSplit(vm.W() + 1)
				Log("    INCW becomes %x", vm.W())
			case 2:
				vm.a--
				Log("    DECA becomes %x", vm.a)
			case 3:
				vm.b, vm.h, vm.l = BhlSplit(vm.W() - 1)
				Log("    DECW becomes %x", vm.W())
			}
		case 0x0C: // BNZ
			if r != 0 {
				return false // undefined instructions
			}
			if vm.a == 0 {
				Log("    BNZ (not taken, to %x)", vm.W())
			} else {
				Log("    BNZ ... branching to %x", vm.W())
				vm.pc = vm.W()
			}
		default:
			return false // undefined instructions
		}
	case 0x01: // MV
		from, to := 7&(t>>3), 7&t
		val := vm.GetReg(from)
		Log("    MV value $%x from %s to %s", val, RegNames[from], RegNames[to])
		vm.PutReg(to, val)
	case 0x10: // LDr
		to, addr := 3&(t>>4), 15&t
		val := vm.ram[addr]
		Log("    LD%x value $%x from addr $%x", RegNames[to], val, addr)
		vm.PutReg(to, val)
	case 0x11: // STr
		from, addr := 3&(t>>4), 15&t
		val := vm.GetReg(from)
		Log("    ST%x value $%x to addr $%x", RegNames[from], val, addr)
		vm.ram[addr] = val
	default:
		panic("bad t")
	}
	return true
}

func BhlSplit(w uint) (b, h, l byte) {
	b, h, l = byte(w>>16), byte(w>>8), byte(w)
	return
}

func BhlJoin(b, h, l byte) uint {
	return (uint(b) << 16) | (uint(h) << 8) | uint(l)
}
