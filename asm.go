package ABhL // pronounced "owl"

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type AddrData struct {
	addr uint
	data byte
}

// Mod is an assembly module
type Mod struct {
	rows []*Row

	labels  map[string]*Label
	macros  map[string]*Macro
	listing io.Writer

	generated []AddrData
}

type Macro struct {
	formals []string
	rows    []*Row
}

// Row is parsed info about an assembly line
type Row struct {
	label   string
	opcode  string
	args    []string
	comment string

	instr *Instr

	length uint
	addr   uint
	final  bool // is addr final?
	// addrX  *Expr  // is addr computed?
	where string
}

type Instr struct {
	length   uint
	generate func(*Mod, *Row)
}

type Expr struct { // TODO
}

type Label struct {
	addr uint
	// addrX  *Expr  // is addr computed?
}

func (mod *Mod) Gen(row *Row, addr uint, value byte) {
	mod.generated = append(mod.generated, AddrData{addr, value})
	mod.ShowGen(row, addr, value)
}

func (mod *Mod) ShowGen(row *Row, addr uint, value byte) {
	if mod.listing != nil {
		if row == nil {
			fmt.Fprintf(mod.listing, "% 6x : %2x\n", addr, value)
		} else {
			fmt.Fprintf(mod.listing, "% 6x : %2x : %12s %12s  %-24s %s\n", addr, value, row.label, row.opcode, strings.Join(row.args, ", "), row.comment)
		}

	}
}
func (mod *Mod) ShowGenPseudo(row *Row, addr uint) {
	if mod.listing != nil {
		fmt.Fprintf(mod.listing, "% 6x : %2s : %12s %12s  %-24s %s\n", addr, "", row.label, row.opcode, strings.Join(row.args, ", "), row.comment)
	}
}

var Instructions = map[string]*Instr{
	"rmb": {0, func(mod *Mod, row *Row) {
		// nothing generated
		mod.ShowGenPseudo(row, row.addr)
	}},
	"org": {0, func(mod *Mod, row *Row) {
		// nothing generated
		mod.ShowGenPseudo(row, row.addr)
	}},
	"equ": {0, func(mod *Mod, row *Row) {
		// nothing generated
		mod.ShowGenPseudo(row, row.addr)
	}},
	"fcw": {2, func(mod *Mod, row *Row) {
		value := mod.EvalArg(row, 0)
		mod.Gen(row, row.addr, byte(value>>8))
		mod.Gen(nil, row.addr+1, byte(value))
	}},
	"fcb": {1, func(mod *Mod, row *Row) {
		value := mod.EvalArg(row, 0)
		mod.Gen(row, row.addr, byte(value))
	}},
	"mv": {1, func(mod *Mod, row *Row) {
		from := mod.GetArgReg(row, 0)
		to := mod.GetArgReg(row, 1)
		mod.Gen(row, row.addr, 0x40+byte((from<<3)+to))
	}},
	"lda": {1, func(mod *Mod, row *Row) {
		value := mod.EvalArg(row, 0)
		mod.Gen(row, row.addr, 0x80+byte(15&value))
	}},
	"ldb": {1, func(mod *Mod, row *Row) {
		value := mod.EvalArg(row, 0)
		mod.Gen(row, row.addr, 0x90+byte(15&value))
	}},
	"ldh": {1, func(mod *Mod, row *Row) {
		value := mod.EvalArg(row, 0)
		mod.Gen(row, row.addr, 0xA0+byte(15&value))
	}},
	"ldl": {1, func(mod *Mod, row *Row) {
		value := mod.EvalArg(row, 0)
		mod.Gen(row, row.addr, 0xB0+byte(15&value))
	}},
	"sta": {1, func(mod *Mod, row *Row) {
		value := mod.EvalArg(row, 0)
		mod.Gen(row, row.addr, 0xC0+byte(15&value))
	}},
	"stb": {1, func(mod *Mod, row *Row) {
		value := mod.EvalArg(row, 0)
		mod.Gen(row, row.addr, 0xD0+byte(15&value))
	}},
	"sth": {1, func(mod *Mod, row *Row) {
		value := mod.EvalArg(row, 0)
		mod.Gen(row, row.addr, 0xE0+byte(15&value))
	}},
	"stl": {1, func(mod *Mod, row *Row) {
		value := mod.EvalArg(row, 0)
		mod.Gen(row, row.addr, 0xF0+byte(15&value))
	}},
	"seta": {2, func(mod *Mod, row *Row) {
		mod.Gen(row, row.addr, 0x04)
		value := mod.EvalArg(row, 0)
		mod.Gen(nil, row.addr+1, byte(value))
	}},
	"setb": {2, func(mod *Mod, row *Row) {
		mod.Gen(row, row.addr, 0x05)
		value := mod.EvalArg(row, 0)
		mod.Gen(nil, row.addr+1, byte(value))
	}},
	"seth": {2, func(mod *Mod, row *Row) {
		mod.Gen(row, row.addr, 0x06)
		value := mod.EvalArg(row, 0)
		mod.Gen(nil, row.addr+1, byte(value))
	}},
	"setl": {2, func(mod *Mod, row *Row) {
		mod.Gen(row, row.addr, 0x07)
		value := mod.EvalArg(row, 0)
		mod.Gen(nil, row.addr+1, byte(value))
	}},
	"inca": {1, func(mod *Mod, row *Row) {
		mod.Gen(row, row.addr, 0x08)
	}},
	"deca": {1, func(mod *Mod, row *Row) {
		mod.Gen(row, row.addr, 0x09)
	}},
	"incw": {1, func(mod *Mod, row *Row) {
		mod.Gen(row, row.addr, 0x0A)
	}},
	"decw": {1, func(mod *Mod, row *Row) {
		mod.Gen(row, row.addr, 0x0B)
	}},
	"bnz": {1, func(mod *Mod, row *Row) {
		mod.Gen(row, row.addr, 0x0C)
	}},
}

func (mod *Mod) GetArgReg(row *Row, i int) uint {
	if len(row.args) < i+1 {
		log.Panicf("not enough args: %d: %#v", i, row)
	}
	argStr := strings.TrimSpace(row.args[i])
	if argStr == "" {
		log.Panicf("empty arg %d: %#v", i, row)
	}
	switch strings.ToLower(argStr) {
	case "a":
		return 0
	case "b":
		return 1
	case "h":
		return 2
	case "l":
		return 3
	case "m":
		return 4
	case "e":
		return 5
	case "f":
		return 6
	case "g":
		return 7
	default:
		log.Panicf("Unknown register %q in arg %d in row %q", argStr, i, row)
		panic(0)
	}
}

func (mod *Mod) EvalPrim(row *Row, s string) uint {
	s = strings.TrimSpace(s)
	if s == "" {
		log.Panicf("Cannot parse empty prim in row: %#v", row)
	}
	var value int64
	var err error
	s0 := s[0]
	if s0 == '$' {
		value, err = strconv.ParseInt(s[1:], 16, 64)
		if err != nil {
			log.Panicf("cannot parse %q as hex int: %#v", s, row)
		}
	} else if s0 == '\'' {
		value = int64(s[1])
	} else if '0' <= s0 && s0 <= '9' {
		value, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			log.Panicf("cannot parse %q as decimal int: %#v", s, row)
		}
	} else {
		lbl, ok := mod.labels[s]
		if !ok {
			log.Panicf("Unknown label %q in row: %#v", s, row)
		}
		value = int64(lbl.addr)
	}
	return 0xFFFFFF & uint(value)
}

var SumPattern = regexp.MustCompile(
	"^[[:space:]]*" + //
		"([$]?[[:word:]]+|'.)" + // group 1: initial Prim
		"[[:space:]]*" + //
		"([-+]?)" + // group 2: Plus or Minus or Empty
		"[[:space:]]*" + //
		"([$]?[[:word:]]+)?" + // group 3: next Prim
		"[[:space:]]*" + //
		"(.*)" + // group 4: all the rest
		"$")

func (mod *Mod) EvalString(row *Row, s string) uint {
	m := SumPattern.FindStringSubmatch(s)
	if m == nil {
		log.Panicf("Eval cannot recognize string %q in row: %#v", s, row)
	}
	for len(m) < 5 {
		m = append(m, "")
	}
	left, op, right, rest := m[1], m[2], m[3], m[4]
	result := mod.EvalPrim(row, left)

	switch op {
	case "":
		if rest != "" {
			log.Panic("Syntax error in expression %q in row: %#v", s, row)
		}
	case "+":
		rv := mod.EvalPrim(row, right)
		t := fmt.Sprintf("$%x %s", result+rv, rest)
		result = mod.EvalString(row, t)
	case "-":
		rv := mod.EvalPrim(row, right)
		t := fmt.Sprintf("$%x %s", result-rv, rest)
		result = mod.EvalString(row, t)
	}

	return result & 0xFFFFFF
}

func (mod *Mod) EvalArg(row *Row, i int) uint {
	if len(row.args) < i+1 {
		log.Panicf("not enough args: %d: %#v", i, row)
	}
	argStr := strings.TrimSpace(row.args[i])
	if argStr == "" {
		log.Panicf("empty arg %d: %#v", i, row)
	}
	return mod.EvalString(row, argStr)
}

var LinePattern = regexp.MustCompile(
	"^" +
		"([A-Za-z0-9_@]*)[:]?" + // group 1: label
		"[\t ]*" +
		"([A-Za-z0-9_]*)" + // group 2: opcode
		"[\t ]*" +
		"([^;]*)" + // group 3: args
		"([;].*)?" + // group 4: comment
		"$")

func LogRows(mod *Mod) {
	for i, row := range mod.rows {
		log.Printf("ROW[%4d]: %#v", i, *row)
	}
}

// PassThree generates code.
func PassThree(mod *Mod) {
	for i, row := range mod.rows {
		if row.opcode == "" {
			if mod.listing != nil {
				if row.label == "" {
					fmt.Fprintf(mod.listing, "%6s   %2s   %12s %s\n", "", "", "", row.comment)
				} else {
					fmt.Fprintf(mod.listing, "% 6x   %2s   %12s %s\n", row.addr, "", row.label, row.comment)
				}
			}
			continue
		}
		if !row.final {
			log.Panicf("Row %d not final: %#v", i, row)
		}
		if row.instr != nil && row.instr.generate != nil {
			row.instr.generate(mod, row)
		}
	}
}

// PassTwo assigns addresses.
func PassTwo(mod *Mod) {
	addr := uint(0)
	for _, row := range mod.rows {
		if row.length == 0 && row.opcode != "" {
			// Special Cases: ORG, RMB...
			if row.opcode == "org" {
				if len(row.args) < 1 {
					log.Panicf("Pseudo-opcode ORG needs an argument, in row: %#v", row)
				}
				addr = mod.EvalPrim(row, row.args[0])
				row.addr = addr
				row.final = true // addr is final
				if row.label != "" {
					lab := mod.labels[row.label]
					lab.addr = addr
				}
			} else if row.opcode == "rmb" {
				if len(row.args) < 1 {
					log.Panicf("Pseudo-opcode RMB needs an argument, in row: %#v", row)
				}
				row.length = mod.EvalPrim(row, row.args[0])
				row.addr = addr
				row.final = true // addr is final
				if row.label != "" {
					lab := mod.labels[row.label]
					lab.addr = addr
				}
				addr += row.length
			} else if row.opcode == "equ" {
				if len(row.args) < 1 {
					log.Panicf("Pseudo-opcode EQU needs an argument, in row: %#v", row)
				}
				row.addr = mod.EvalPrim(row, row.args[0])
				row.final = true // addr is final
				if row.label != "" {
					lab := mod.labels[row.label]
					lab.addr = row.addr
				}
			} else {
				log.Panicf("Uknown pseudo-opcode %q has 0 length, in row: $#v", row.opcode, row)
			}
		} else {
			// Normal fixed-length instructions
			// and non-generating lines (like just a label or comment)
			row.addr = addr
			row.final = true // addr is final
			if row.label != "" {
				lab := mod.labels[row.label]
				lab.addr = addr
			}
			addr += row.length
		}
	}
}

// PassOne creates labels and looks up instructions by opcode.
func PassOne(mod *Mod) {
	for i, row := range mod.rows {
		if row.label != "" {
			mod.labels[row.label] = &Label{}
		}
		if row.opcode == "" {
			continue
		}
		instr, ok := Instructions[row.opcode]
		if !ok {
			log.Panicf("Unknown opcode on line %d: %q", i+1, row.opcode)
		}
		row.instr = instr
		row.length = instr.length
	}
}

// MacroPassTwo expands macros.
func MacroPassTwo(mod *Mod) {
	serial := 1
	unique := fmt.Sprintf("B%d__", serial)

	var newRows []*Row
	for _, row := range mod.rows {
		// Totally empty lines get a new Serial.
		if row.label == "" && row.opcode == "" && row.comment == "" {
			serial++
			unique = fmt.Sprintf("B%d__", serial)
		}

		macro, ok := mod.macros[row.opcode]
		if ok {
			saved := unique
			serial++
			unique = fmt.Sprintf("M%d__", serial)

			newRows = append(newRows, &Row{
				comment: fmt.Sprintf("; Expanded macro %s ( %s )",
					row.opcode,
					strings.Join(row.args, ", ")),
			})
			for _, innerRow := range macro.rows {
				// Append normal non-macro rows to newRows.
				var innerCopy Row = *innerRow // struct assignment makes a copy

				for i, formal := range macro.formals {
					param := row.args[i]
					innerCopy.label = strings.Replace(innerCopy.label, formal, param, -1)
					innerCopy.label = strings.Replace(innerCopy.label, "@", unique, -1)
					innerCopy.opcode = strings.Replace(innerCopy.opcode, formal, param, -1)
					var newArgs []string
					for _, arg := range innerCopy.args {
						arg = strings.Replace(arg, formal, param, -1)
						arg = strings.Replace(arg, "@", unique, -1)
						newArgs = append(newArgs, arg)
					}
					innerCopy.args = newArgs
				}

				newRows = append(newRows, &innerCopy)
			}

			newRows = append(newRows, &Row{
				comment: "; End Expansion",
			})

			unique = saved
		} else {
			// Append normal non-macro rows to newRows.
			row.label = strings.Replace(row.label, "@", unique, -1)
			var newArgs []string
			for _, arg := range row.args {
				newArgs = append(newArgs, strings.Replace(arg, "@", unique, -1))
			}
			row.args = newArgs
			newRows = append(newRows, row)
		}
	}
	mod.rows = newRows // with the macros expanded
}

// MacroPassOne creates macros.
func MacroPassOne(mod *Mod) {
	var macro *Macro

	var newRows []*Row
	for _, row := range mod.rows {
		if macro == nil {
			// Outside of a macro definition
			if row.opcode == "macro" {
				// Starts a macro definition
				if row.label == "" {
					log.Panicf("Cannot define a macro without a label: %#v", row)
				}
				macro = &Macro{
					formals: row.args,
				}
				mod.macros[strings.ToLower(row.label)] = macro

				newRows = append(newRows, &Row{
					comment: fmt.Sprintf("; MACRO DEFINITION"),
				})
				newRows = append(newRows, &Row{
					comment: fmt.Sprintf("; %12s %12s  %-20s %s",
						row.label, row.opcode,
						strings.Join(row.args, ", "), row.comment),
				})
			} else {
				// Append normal non-macro rows to newRows.
				newRows = append(newRows, row)
			}
		} else {
			// defining a macro
			if row.opcode == "endmacro" {
				// ends a macro definition
				macro = nil
			} else {
				// another line in the macro
				macro.rows = append(macro.rows, row)
			}
			newRows = append(newRows, &Row{
				comment: fmt.Sprintf("; %12s %12s %-20s %s",
					row.label, row.opcode,
					strings.Join(row.args, ", "), row.comment),
			})
		}
	}
	mod.rows = newRows // with the macros definitions removed.
}

func SplitOnCommaAndTrim(args string) (vec []string) {
	for _, a := range strings.Split(args, ",") {
		vec = append(vec, strings.TrimSpace(a))
	}
	return
}

func ParseLine(line string) *Row {
	line = strings.TrimRight(line, "\r\n")
	m := LinePattern.FindStringSubmatch(line)
	if m == nil {
		log.Fatalf("Cannot parse line: %q", line)
	}
	for len(m) < 5 {
		m = append(m, "")
	}
	label, opcode, args, comment := m[1], m[2], m[3], m[4]
	// Log("Parsed (%q, %q, %q, %q) <- %q", label, opcode, args, comment, line)

	row := &Row{
		label:   label,
		opcode:  strings.ToLower(opcode),
		args:    SplitOnCommaAndTrim(args),
		comment: comment,
	}
	// Log("      Row -> %#v", *row)
	return row
}
func ParseLines(lines []string, wheres []string) *Mod {
	mod := &Mod{
		labels:  make(map[string]*Label),
		macros:  make(map[string]*Macro),
		listing: os.Stdout,
	}
	for i, line := range lines {
		row := ParseLine(line)
		row.where = wheres[i]
		mod.rows = append(mod.rows, row)
	}
	return mod
}

func SlurpTextFile(filename string) (lines []string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Panicf("Cannot SlurpTextFile %q: %v", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return
}

func CreateIPL(mod *Mod) []byte {
	var ipl []byte
	current := uint(0xFFFFFFFF) // will not match any addr

	for _, pair := range mod.generated {
		if pair.addr == current {
			ipl = append(ipl, 0x0A /*incw*/, 0)
		} else {
			b, h, l := BhlSplit(pair.addr)
			ipl = append(ipl,
				0x05 /*setb*/, b,
				0x06 /*seth*/, h,
				0x07 /*setl*/, l)
		}
		ipl = append(ipl,
			0x04 /*seta*/, pair.data,
			0x44 /*mv a,m*/, 0)
		current = pair.addr + 1
	}

	//if false {
	//	for i := uint(0); i < RamSize; i++ {
	//		a := mod.ram[i]
	//		if a != 0 {
	//			b, h, l := BhlSplit(i)
	//			ipl = append(ipl,
	//				0x04 /*seta*/, a,
	//				0x05 /*setb*/, b,
	//				0x06 /*seth*/, h,
	//				0x07 /*setl*/, l,
	//				0x44 /*mv a,m*/, 0,
	//				0x44 /*mv a,m*/, 0, // do it 4 times,
	//				0x44 /*mv a,m*/, 0, // just so "hd ipl" looks prettier.
	//				0x44 /*mv a,m*/, 0)
	//		}
	//	}
	//}

	start := uint(0)
	start_label, ok := mod.labels["start"]
	if ok {
		start = start_label.addr
	}

	b, h, l := BhlSplit(start)
	ipl = append(ipl,
		0x04 /*seta*/, 1, // enable jump
		0x05 /*setb*/, b, // start address
		0x06 /*seth*/, h,
		0x07 /*setl*/, l,
		0x0C /*bnz*/, 0,
		0x0C /*bnz*/, 0, // do it 4 times,
		0x0C /*bnz*/, 0, // just so "hd ipl" looks prettier.
		0x0C /*bnz*/, 0)
	return ipl
}

func WriteIPL(mod *Mod, filename string) {
	err := ioutil.WriteFile(filename, CreateIPL(mod), 0644)
	if err != nil {
		log.Panicf("Error writing IPL file %q: %v", filename, err)
	}
}
