# The ABhL (pronounced "owl") CPU Architecture

Henry Strickland  -- July 2024 -- strick@yak.net

ABhL is retro computing architecture designed with a minimal set of
instructions for simple implementation in TTL or CMOS.  It has an 8-bit
data bus and 24-bit address bus, but you would call it an 8-bit CPU.

Most of the chips needed would have been available (in some form) in the
1970's or 1980's.  The big exception is the big static RAM.  A standard
ABhL computer has one megabyte of RAM (but it can address up to 16MB).
But the machine has no ALU, so arithmetic is done by looking up facts
in tables in RAM.  Those tables are considered part of the program.

There is an Iniital Program Load ("IPL", or "boot") facility for feeding
your program into the machine in a standard way.
When we make PCBs, there will probably be a slot for a `Raspbery Pi Pico W`
to be used for driving the IPL.  It may also be used for sideline
activity like driving the clock signals and simulating an I/O device.
Otherwise there are no programmable logic chips like PALs, GALs, CPLDs,
or FPGAs.

## Primary Registers

There are four primary 8-bit registers named A, B, H, and L:

* `A: Accumulator byte`
* `B: Bank address byte (most significant)  }`
* `H: High address byte                     }  also called W (24 bits)`
* `L: Low address byte (least significant)  }`

A is the accumulator.  B, H, and L are usually used together, and often hold a 24-bit address.
B is the most significant byte of the address; it is named for a "bank"
of 64k bytes.  (A 1MB RAM has 16 banks.)  H and L are the High and Low
byte address within a bank.  When used together, B H and L are sometimes
called W, which is thought of as a 24 bit number.

There are instructions to set any of these four primary registers
to an immediate value from 0 to 255.

## Secondary Registers

Four other registers or pseudo-registers are also on the data bus
with the four primary registers:

* M: the byte in RAM currently addressed by W.
* E: an I/O port
* F: an I/O port (conventionally, a terminal session)
* G: an I/O port

There are instructions to move any of the 8 registers A, B, H, L, M, E, F, and G
to and from each other.

## Quick Registers

The first 16 bytes of RAM are also considered registers.
These quick registers can be called Q0 through Q15.

There are instructions to move data between
any of the 4 primary registers A, B, H, or L,
and any of the 16 quick registers.

## Program Counter

The program counter is a 24-byte register.

There is only one instruction to alter the program
counter from its usual behavior of just incrementing.
That is a BNZ (branch if accumulator A is not zero)
instruction, that copies W to PC, but only if A is nonzero.

## Ram Divisions

A group of 64K bytes that have the same number in the B part
of the address are called a BANK.

A group of 256 bytes that have the same numbeers in the B and H
part of their address are called a ROW.

The assembler has special pseudo-ops for allocating and initializing
BANKs and ROWs.  Significant optimizations can be achieved by smarty using
BANKs and ROWs.  Arithmetic and Logic tables should also be organized by
BANKs and ROWs.

## The instructions

All opcodes are 8 bits.  Only the SET instructions
take an extra 8 bits as an immediate argument.

The machine alternately executes FETCH cycles
and EXECUTE cycles.  Each FETCH cycles fetches 
a one-byte opcode from memory at the program counter
(and the program counter is incremented).
Exactly one EXECUTE cycle follows each FETCH cycle.

A special (unaddressable) latch named T remembers the opcode fetched
on the final edge of the FETCH cycle clock.  The opcode in T is decoded
and executed during the EXECUTE clock cycle.

The results of executing an opcode are latched synchronously on the
final edge of the EXECUTE cycle clock.

The instructions below (such as MV) will be presented with their binary
opcode pattern (such as 01fffttt).

### MV (01fffttt)

The instruction copies 8 bits of data from one
register (the fff bits) to another register
(the ttt bits).  (It does not make sense,
and will not work, to transfer to and from the same register.)

The three bit codes for the registers (fff and ttt) are

* `000: A`
* `001: B`
* `010: H`
* `011: L`
* `100: M`
* `101: E`
* `110: F`
* `111: G`

The opcode mnemonic takes two register names as arguments.
The first is the source, and the later is the destination.

e.g. `MV m,a` means "move the byte from memory (addressed
by W) to the accumulator".

### LD (10rrqqqq)

Load one of the four primary registers (rr) from quick register (qqqq).
The mnemonics for the opcodes include the primary register in the name,
and take the quick register number as an argument.

* `1000qqqq:  LDA`
* `1001qqqq:  LDB`
* `1010qqqq:  LDH`
* `1011qqqq:  LDL`

`/* Update: The rr and qqqq bits will be swapped for easier decoding */`

### ST (11rrqqqq)

Store one of the four primary registers (rr) to quick register (qqqq).
The mnemonics for the opcodes include the primary register in the name,
and take the quick register number as an argument.

* `1100qqqq:  STA`
* `1101qqqq:  STB`
* `1110qqqq:  STH`
* `1111qqqq:  STL`

### SET (000001rr)

Set primary register (rr) to the byte immediately following
the instruction.  The program counter will naturally be incremented
by two before fetching the next opcode.
The mnemonics for the opcodes include the primary register in the name,
and take the immediate byte value as an argument.

* `00000100:  SETA`
* `00000101:  SETB`
* `00000110:  SETH`
* `00000111:  SETL`

Try not to confuse ST with SET instructions, the names look similar!

### INCA (00001000)

Increment the A register by 1.
If A is 0xFF, it rolls around back to 0.

### INCW (00001010)

Increment the W register by 1.  Notice that there is a ripple carry
from L to H, and from H to B.
If W is 0xFFFFFF, it rolls around back to 0.

### DECA (00001001) (optional instruction)

Decrement the A register by 1.
A standard machine may not have this instruction.

### DECW (00001011) (optional instruction)

Decrement the W register by 1.  Notice that there is a ripple carry
from L to H, and from H to B.
A standard machine may not have this instruction.

### BNZ (00001100)

Branch if A is not zero.  If any of the bits in A are set,
copy the W register to the program counter.  That is where
the next instruction will be fetched from.

`/* Update: The mnemonic might be renamed JNZ, jump if A is not zero */`

`/* Update: It may be trivial to add 00001101 JMP: jump always */`

### STOP (00000000) (optional instruction)

The emulator will stop if it hits a 00000000 instruction.
That event probably means it is executing uninitialized
memory, which is not useful behaviour.

Hardware implementations may not have this instruction.

## TODO: add a diagram.
## TODO: describe the cycles.
