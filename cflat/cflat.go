package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
)

func main() {
	flag.Parse()

	for _, arg := range flag.Args() {
		Compile(arg)
	}
}

func Compile(filename string) {
	bb := Value(ioutil.ReadFile(filename))
	w := Value(os.Create(filename + ".genowl"))
	src := string(bb)
	src = strings.Replace(src, "\t", " ", -1)
	par := &Parser{
		filename: filename,
		src:      src,
		remain:   strings.Replace(src, "\n", ";", -1),

		vars:  make(map[string]bool),
		rows:  make(map[string]bool),
		banks: make(map[string]bool),
		funcs: make(map[string]*Func),

		w: w,
	}
	par.Next() // move to initial token
	par.Parse()
	par.Header()
	par.Generate()
}

const (
	KEYWORD = iota
	NUMBER
	IDENT
	BINOP
	PUNC
	EOF
)

type Parser struct {
	// Lexing part.
	filename string
	src      string
	remain   string
	tok      string
	typ      int

	// Parser State
	vars  map[string]bool
	rows  map[string]bool
	banks map[string]bool
	funcs map[string]*Func

	// Code Generation
	w   io.Writer
	qsp int // Quick Stack Pointer
}

var Lexers = []struct {
	typ     int
	pattern *regexp.Regexp
}{
	{KEYWORD, regexp.MustCompile(`^[[:space:]]*(func|var|bank|row|while|return)\b(.*)$`)},
	{NUMBER, regexp.MustCompile(`^[[:space:]]*(0x[0-9a-fA-F]+|[0-9]+|'.')(.*)$`)},
	{IDENT, regexp.MustCompile(`^[[:space:]]*([[:word:]]+)(.*)$`)},
	{BINOP, regexp.MustCompile(`^[[:space:]]*([+][+]|[-][-]|[-+*/%^|&]|<<|>>|==|!=|<=|>=|<|>)(.*)$`)},
	{PUNC, regexp.MustCompile(`^[[:space:]]*([(){}=,;])(.*)$`)},
	{EOF, regexp.MustCompile(`^[[:space:]]*()()$`)},
}

func (par *Parser) Next() {
	for _, pair := range Lexers {
		Log("Try pattern %d : %q ======= %q", pair.typ, pair.pattern, par.remain)
		m := pair.pattern.FindStringSubmatch(par.remain)
		if m != nil {
			par.typ = pair.typ
			par.tok = m[1]
			par.remain = m[2]
			Log("match: %d %q", par.typ, par.tok)
			return
		}
	}
	par.Fail("cannot recognize next token")
}

func (par *Parser) LineNo() int {
	n := len(par.src) - len(par.remain)
	count := 1 // first line is line 1
	for i := 0; i < n; i++ {
		if par.src[i] == '\n' {
			count++
		}
	}
	return count
}

type Expr struct {
	konst *uint
	vari  string

	subj *Expr
	args []*Expr
	op   string
}

// QStack gives the Q pair for the corresponding Quick Stack position,
// where 1=top, 2=nextToTop, 3=...
func (par *Parser) QStack(i int) int {
	return 2*par.qsp - i
}
func (par *Parser) QPop() int {
	par.qsp--
	if par.qsp < 0 {
		par.Fail("Quick Stack Underflow")
	}
	return par.QStack(1)
}
func (par *Parser) QPush() int {
	par.qsp++
	if par.qsp > 8 {
		par.Fail("Quick Stack Overflow")
	}
	return par.QStack(1)
}

func (par *Parser) EvaluateVar(fn *Func, ex *Expr) string {
	if ex.vari != "" {
		name := ex.vari
		if Contains(fn.args, name) || Contains(fn.locals, name) {
			name = Fmt("%s.var.%s", fn.id, name)
		}
		return name
	}
	panic(par.Fail("Should be a variable name: %#v", ex))
}

/*********************
func (par *Parser) GenerateExpr(fn *Func, ex *Expr) (z []string) {
	// Create a new stack slot for the result.
	q := par.QPush()

	z = append(z, Fmt("; q=%d EvaluateExpr(%#v)", q, *ex))
	if ex.konst != nil {
		z = append(z,
			Fmt("  setw $%x", *ex.konst),
			Fmt("  sthl q%d", q),
		)
	} else if ex.vari != "" {
		name := ex.vari
		if Contains(fn.args, name) || Contains(fn.locals, name) {
			name = Fmt("%s.var.%s", fn.id, name)
		}
		z = append(z,
			Fmt("  setw %s", name),
			"  mv m,a",
			Fmt("  sta q%d", q),
			"  incw",
			"  mv m,a",
			Fmt("  sta q%d", q+1),
			";",
		)
	} else {
		switch ex.op {
		case "":
			par.EvaluateExpr(fn, ex.subj)
		case "++":
			varName := par.EvaluateVar(fn, ex.subj)
			z = append(z,
				Fmt("  setw %s", varName),
				"  mv m,a",
				"  incw",
				"  mv m,l",
				"  mv a,h",
				"  incw",
				Fmt("  sthl q%d", q),
				Fmt("  setw %s", varName),
				Fmt("  lda q%d", q),
				"mv a,m",
				"  incw",
				Fmt("  lda q%d", q+1),
				"mv a,m",
				";",
			)
		case "+":
			par.EvaluateExpr(fn, ex.subj)
			q1 := par.QStack(1)
			par.EvaluateExpr(fn, ex.args[0])
			q2 := par.QStack(1)
			z = append(z,
				Fmt("  add2qqq q%d,q%d,q%d", q1, q2, q),
			)
			par.QPop()
			par.QPop()
		default:
			panic(ex.op)
		}

	}
	return
}
************/

type Stmt struct {
	kind  string
	guard *Expr
	body  []*Stmt

	value  *Expr
	assign string
}

type Func struct {
	id     string
	args   []string
	locals []string
	body   []*Stmt
}

func (par *Parser) DefineFunc(id string) {
	fn := &Func{
		id: id,
	}
	par.funcs[id] = fn

	par.Take("(")
	for par.typ == IDENT || par.tok == "," { // args
		if par.typ == IDENT {
			fn.args = append(fn.args, par.tok)
		}
		par.Take(par.tok)
	}
	par.Take(")")
	for par.typ == IDENT || par.tok == "," { // locals
		if par.typ == IDENT {
			fn.locals = append(fn.locals, par.tok)
		}
		par.Take(par.tok)
	}
	par.Take("{")
	fn.body = par.ParseBody(fn)
	par.Take("}")
}

func (par *Parser) TakeNumber() uint {
	s := par.tok

	var value int64
	var err error
	s0 := s[0]
	if s0 == '$' {
		value, err = strconv.ParseInt(s[1:], 16, 64)
		if err != nil {
			par.Fail("cannot parse %q as hex int", s)
		}
	} else if strings.HasPrefix(s, "0x") {
		value, err = strconv.ParseInt(s[2:], 16, 64)
		if err != nil {
			par.Fail("cannot parse %q as hex int", s)
		}
	} else if strings.HasPrefix(s, "0o") {
		value, err = strconv.ParseInt(s[2:], 8, 64)
		if err != nil {
			par.Fail("cannot parse %q as octal int", s)
		}
	} else if strings.HasPrefix(s, "0b") {
		value, err = strconv.ParseInt(s[2:], 2, 64)
		if err != nil {
			par.Fail("cannot parse %q as binary int", s)
		}
	} else if '0' == s0 {
		value, err = strconv.ParseInt(s, 8, 64)
		if err != nil {
			par.Fail("cannot parse %q as octal int", s)
		}
	} else if '1' <= s0 && s0 <= '9' {
		value, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			par.Fail("cannot parse %q as decimal int", s)
		}
	} else if s0 == '\'' {
		value = int64(s[1])
	} else {
		par.Fail("cannot parse %q as a number", s)
	}
	par.Take(s)
	return 0xFFFFFF & uint(value)
}

func (par *Parser) ParsePrimX(fn *Func) (x *Expr) {
	switch par.tok {
	case "(":
		par.Take("(")
		x = par.ParseExpr(fn)
		par.Take(")")
		return x
	case "peek":
		var bx, hx, lx *Expr
		par.Take("(")
		if par.tok != ":" {
			bx = par.ParseExpr(fn)
		}
		par.Take(":")
		hx = par.ParseExpr(fn)
		par.Take(":")
		lx = par.ParseExpr(fn)
		par.Take(")")
		return &Expr{
			op:   "peek",
			args: []*Expr{bx, hx, lx},
		}
	case "poke":
		var bx, hx, lx, val *Expr
		par.Take("(")
		if par.tok != ":" {
			bx = par.ParseExpr(fn)
		}
		par.Take(":")
		hx = par.ParseExpr(fn)
		par.Take(":")
		lx = par.ParseExpr(fn)
		par.Take(",")
		val = par.ParseExpr(fn)
		par.Take(")")
		return &Expr{
			op:   "poke",
			args: []*Expr{bx, hx, lx, val},
		}
	}
	switch par.typ {
	case NUMBER:
		val := par.TakeNumber()
		return &Expr{
			konst: &val,
		}
	case IDENT:
		id := par.TakeIdent()
		return &Expr{
			vari: id,
		}
	default:
		panic(par.Fail("unknown primative: %q", par.tok))
	}
}

func (par *Parser) ParseCallX(fn *Func) *Expr {
	x := par.ParsePrimX(fn)
	switch par.tok {
	case "(":
		var args []*Expr
		par.Take("(")
		for par.tok != ")" {
			if par.tok == "," {
				par.Take(",")
				continue
			}
			a := par.ParseExpr(fn)
			args = append(args, a)
		}
		par.Take(")")
		return &Expr{
			op:   "call",
			subj: x,
			args: args,
		}
	default:
		return x
	}
}

func (par *Parser) ParseProductX(fn *Func) *Expr {
	x := par.ParseCallX(fn)
	op := par.tok
	for {
	SWITCH:
		switch par.tok {
		case "*":
			break SWITCH
		case "/":
			break SWITCH
		case "%":
			break SWITCH
		default:
			return x
		}
		par.Take(op)
		y := par.ParseCallX(fn)
		x = &Expr{
			op:   op,
			subj: x,
			args: []*Expr{y},
		}
	}
}

func (par *Parser) ParseSumX(fn *Func) *Expr {
	x := par.ParseProductX(fn)
	op := par.tok
	for {
	SWITCH:
		switch par.tok {
		case "+":
			break SWITCH
		case "-":
			break SWITCH
		default:
			return x
		}
		par.Take(op)
		y := par.ParseProductX(fn)
		x = &Expr{
			op:   op,
			subj: x,
			args: []*Expr{y},
		}
	}
}

func (par *Parser) ParseRelativeX(fn *Func) *Expr {
	x := par.ParseSumX(fn)
	op := par.tok
	switch par.tok {
	case "==":
		break
	case "!=":
		break
	case "<=":
		break
	case ">=":
		break
	case "<":
		break
	case ">":
		break
	default:
		return x
	}
	par.Take(op)
	y := par.ParseSumX(fn)
	return &Expr{
		op:   op,
		subj: x,
		args: []*Expr{y},
	}
}

func (par *Parser) ParseExpr(fn *Func) (x *Expr) {
	x = par.ParseRelativeX(fn)
	return x
}

func (par *Parser) ParseBody(fn *Func) (body []*Stmt) {
	Log("Parse Body <<<")
LOOP:
	for {
		Log("Parse Body ::: %d : %q", par.typ, par.tok)
		switch par.tok {
		case "if":
			par.Take("if")
			c := par.ParseExpr(fn)
			par.Take("{")
			stuff := par.ParseBody(fn)
			par.Take("}")
			body = append(body, &Stmt{
				kind:  "if",
				guard: c,
				body:  stuff,
			})
		case "while":
			par.Take("while")
			c := par.ParseExpr(fn)
			par.Take("{")
			stuff := par.ParseBody(fn)
			par.Take("}")
			body = append(body, &Stmt{
				kind:  "while",
				guard: c,
				body:  stuff,
			})
		case "break":
			par.Take("break")
			par.TakeEnder()
			body = append(body, &Stmt{kind: "break"})
		case "continue":
			par.Take("continue")
			par.TakeEnder()
			body = append(body, &Stmt{kind: "continue"})
		case "return":
			par.Take("return")
			var zero uint
			r := &Expr{konst: &zero}
			if !par.IsEnder() {
				r = par.ParseExpr(fn)
			}
			par.TakeEnder()
			body = append(body, &Stmt{
				kind:  "return",
				value: r,
			})
		case "++":
			par.Take("++")
			id := par.TakeIdent()
			par.TakeEnder()
			body = append(body, &Stmt{
				kind:   "++",
				assign: id,
			})
		case ";":
			par.Take(";")
			// pass
		case "}":
			break LOOP
		default:
			x := par.ParseExpr(fn)
			switch par.tok {
			case "=":
				par.Take("=")
				assigned := x.vari
				if assigned == "" {
					par.Fail("Bad LHS of assignment: %#v", x)
				}
				y := par.ParseExpr(fn)
				body = append(body, &Stmt{
					kind:   "",
					value:  y,
					assign: assigned,
				})
			case "++":
				par.Take("++")
				assigned := x.vari
				if assigned == "" {
					par.Fail("Bad LHS of ++: %#v", x)
				}
				body = append(body, &Stmt{
					kind:   "++",
					assign: assigned,
				})
			default:
				body = append(body, &Stmt{
					kind:  "",
					value: x,
				})
			}
		}
	} // end LOOP
	Log("Parse Body >>> [%d] %#v", len(body), body)
	return
}

func (par *Parser) IsEnder() bool {
	switch par.tok {
	case ";":
		return true
	case "}":
		return true
	default:
		return false
	}
}
func (par *Parser) TakeEnder() {
	switch par.tok {
	case ";":
		par.Take(";")
		return
	case "}":
		par.Take("}")
		return
	}
	par.Fail("Expected end of statement (semicolon, newline, or close bracket), but got %q", par.tok)
}

func (par *Parser) Take(tok string) {
	if par.tok != tok {
		par.Fail("Expected %q but got %q", tok, par.tok)
	}
	par.Next()
}

func (par *Parser) TakeIdent() string {
	if par.typ != IDENT {
		par.Fail("Expected Identifier but got %q", par.tok)
	}
	id := par.tok
	par.Next()
	return id
}

func (par *Parser) DefineBank(id string) {
	par.banks[id] = true
}
func (par *Parser) DefineRow(id string) {
	par.rows[id] = true
}
func (par *Parser) DefineVar(id string) {
	par.vars[id] = true
}
func (par *Parser) Header() {
	pf := func(f string, args ...any) {
		fmt.Fprintf(par.w, f+"\n", args...)
	}

	pf("RowsBank BANK")
	pf("  org $100")
}
func (par *Parser) Generate() {
	pf := func(f string, args ...any) {
		fmt.Fprintf(par.w, f+"\n", args...)
	}

	for _, id := range SortedKeys(par.banks) {
		pf("%s  BANK  0", id)
	}
	for _, id := range SortedKeys(par.rows) {
		pf("%s  ROW  0", id)
	}
	for _, id := range SortedKeys(par.vars) {
		pf("%s  RMB 2", id)
	}
	for _, id := range SortedKeys(par.funcs) {
		fn := par.funcs[id]
		pf("%s.addr:", id)
		pf("  fcb B(%s.entry)", id)
		pf("  fcb H(%s.entry)", id)
		pf("  fcb L(%s.entry)", id)
		pf("  fcb 0")
		pf("%s.retval  RMB 2", id)
		for i, name := range fn.args {
			pf("%s.var.%s  RMB 2 ; arg[%d]", id, name, i)
		}
		for i, name := range fn.locals {
			pf("%s.var.%s  RMB 2 ; local[%d]", id, name, i)
		}
	}
	for _, id := range SortedKeys(par.funcs) {
		fn := par.funcs[id]
		pf(";")
		pf(";")
		pf("%s.entry:", id)
		pf(";")
		pf("  seta 0  ; initialize locals")
		pf("  setw %s.retval", id)
		pf("  mv a, m")
		pf("  incw")
		pf("  mv a, m")
		for _, name := range fn.locals {
			pf("  setw %s.var.%s", id, name)
			pf("  mv a,m")
			pf("  incw")
			pf("  mv a,m")
		}
		pf(";")
		par.GenerateBody(fn, fn.body)
		pf(";")
		pf("%s.exit:", id)
		pf("%s.retsetb: setb 0", id)
		pf("%s.retseth: seth 0", id)
		pf("%s.retsetl: setl 0", id)
		pf("            seta 1")
		pf("            bnz")
		pf("%s.finished:", id)
		pf(";")
		pf(";")
	}
}

func (par *Parser) GenerateBody(fn *Func, body []*Stmt) {
	for _, it := range body {
		par.GenerateStatement(fn, it)
	}
}
func (par *Parser) GenerateStatement(fn *Func, st *Stmt) {
	pf := func(f string, args ...any) {
		fmt.Fprintf(par.w, f+"\n", args...)
	}

	switch st.kind {
	case "if":
		pf("  ; TODO if")
	case "while":
		pf("  ; TODO while")
	case "break":
		pf("  ; TODO break")
	case "continue":
		pf("  ; TODO continue")
	case "return":
		pf("  ; TODO return")
	case "call":
		pf("  ; TODO call")
	case "peek":
		pf("  ; TODO peek")
	case "poke":
		pf("  ; TODO poke")
	case "":
		pf("  ; TODO no kind")
	default:
		pf("  ; TODO kind? %q", st.kind)
	}
	pf("  ;;;;;  %#v", *st)
}

func (par *Parser) Parse() {
	for par.typ != EOF {
		switch par.tok {
		case "bank":
			par.Next()
			id := par.TakeIdent()
			par.DefineBank(id)
			par.Take(";")
		case "row":
			par.Next()
			id := par.TakeIdent()
			par.DefineRow(id)
			par.Take(";")
		case "var":
			par.Next()
			id := par.TakeIdent()
			par.DefineVar(id)
			par.Take(";")
		case "func":
			par.Next()
			id := par.TakeIdent()
			par.DefineFunc(id)
			par.Take(";")
		case ";":
			par.Next()
		default:
			par.Fail("Unknown token %q at outer level", par.tok)
		}
	}
}
func (par *Parser) Fail(format string, args ...any) bool {
	nr := len(par.remain)
	if nr > 12 {
		nr = 12
	}
	where := Fmt("Parse failure at line %d on token %q before %q: ", par.LineNo(), par.tok, par.remain[:nr])
	strings.Replace(where, "%", "%%", -1)
	log.Panicf(where+format, args...)
	return false
}

////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////
//
//  My Shortcuts for Go

// coco-shelf/gomar/gu/generics.go

// import ( "errors" "fmt" "log" "runtime/debug" )

type Number interface {
	~int8 | ~int16 | ~int32 | ~uint8 | ~uint16 | ~uint32 | ~int | ~uint | ~int64 | ~uint64 | ~uintptr
}
type _comparable interface {
	Number | string
}

func Contains[T _comparable](list []T, value T) bool {
	for _, it := range list {
		if it == value {
			return true
		}
	}
	return false
}

func Cond[T any](a bool, b T, c T) T {
	if a {
		return b
	}
	return c
}

func Min[T Number](b T, c T) T {
	if b < c {
		return b
	}
	return c
}

func Max[T Number](b T, c T) T {
	if b > c {
		return b
	}
	return c
}

func Panicf(f string, args ...any) bool {
	log.Panicf(f, args...)
	return false
}

func Errorf(f string, args ...any) error {
	return errors.New(fmt.Sprintf(f, args...))
}

func Value[T any](value T, err error) T {
	Check(err)
	return value
}

func Assert(b bool, args ...any) {
	if !b {
		s := "Assert Fails"
		for _, x := range args {
			s += fmt.Sprintf(" ; %v", x)
		}
		s += "\n[[[[[[\n" + string(debug.Stack()) + "\n]]]]]]\n"
		log.Panic(s)
	}
}

func AssertEQ[N Number](a, b N, args ...any) {
	if a != b {
		s := fmt.Sprintf("AssertEQ Fails: (%v .EQ. %v)", a, b)
		for _, x := range args {
			s += fmt.Sprintf(" ; %v", x)
		}
		s += "\n[[[[[[\n" + string(debug.Stack()) + "\n]]]]]]\n"
		log.Panic(s)
	}
}

func AssertNE[N Number](a, b N, args ...any) {
	if a == b {
		s := fmt.Sprintf("AssertNE Fails: (%v .NE. %v)", a, b)
		for _, x := range args {
			s += fmt.Sprintf(" ; %v", x)
		}
		s += "\n[[[[[[\n" + string(debug.Stack()) + "\n]]]]]]\n"
		log.Panic(s)
	}
}

func AssertLT[N Number](a, b N, args ...any) {
	if a >= b {
		s := fmt.Sprintf("AssertLT Fails: (%v .LT. %v)", a, b)
		for _, x := range args {
			s += fmt.Sprintf(" ; %v", x)
		}
		s += "\n[[[[[[\n" + string(debug.Stack()) + "\n]]]]]]\n"
		log.Panic(s)
	}
}

func AssertLE[N Number](a, b N, args ...any) {
	if a > b {
		s := fmt.Sprintf("AssertLE Fails: (%v .LE. %v)", a, b)
		for _, x := range args {
			s += fmt.Sprintf(" ; %v", x)
		}
		s += "\n[[[[[[\n" + string(debug.Stack()) + "\n]]]]]]\n"
		log.Panic(s)
	}
}

func AssertGT[N Number](a, b N, args ...any) {
	if a <= b {
		s := fmt.Sprintf("AssertGT Fails: (%v .GT. %v)", a, b)
		for _, x := range args {
			s += fmt.Sprintf(" ; %v", x)
		}
		s += "\n[[[[[[\n" + string(debug.Stack()) + "\n]]]]]]\n"
		log.Panic(s)
	}
}

func AssertGE[N Number](a, b N, args ...any) {
	if a < b {
		s := fmt.Sprintf("AssertGE Fails: (%v .GE. %v)", a, b)
		for _, x := range args {
			s += fmt.Sprintf(" ; %v", x)
		}
		s += "\n[[[[[[\n" + string(debug.Stack()) + "\n]]]]]]\n"
		log.Panic(s)
	}
}

func Check(err error, args ...any) {
	if err != nil {
		s := fmt.Sprintf("Check Fails: %v", err)
		for _, x := range args {
			s += fmt.Sprintf(" ; %v", x)
		}
		s += "\n[[[[[[\n" + string(debug.Stack()) + "\n]]]]]]\n"
		log.Panic(s)
	}
}

func Hex[N Number](x N) string {
	return fmt.Sprintf("$%x", x)
}

//////////////////////////////////////////

// coco-shelf/gomar/gu/shortcuts.go

func Fmt(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func QFmt(format string, args ...any) string {
	return fmt.Sprintf("%q", fmt.Sprintf(format, args...))
}

func Log(format string, args ...any) {
	log.Printf(format, args...)
}

func Str(x any) string {
	return fmt.Sprintf("%v", x)
}

func Repr(x any) string {
	return fmt.Sprintf("%#v", x)
}

func QStr(x any) string {
	return fmt.Sprintf("%q", fmt.Sprintf("%v", x))
}

func QRepr(x any) string {
	return fmt.Sprintf("%q", fmt.Sprintf("%#v", x))
}

func DeHex(s string) uint {
	x, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		log.Panicf("Cannot convert hex $q: %v", s, err)
	}
	return uint(x)
}

/////////////////////////////

func SortedStrings(m []string) (vec []string) {
	vec = m[:]
	sort.Strings(vec)
	return
}

func SortedKeys[T any](m map[string]T) (vec []string) {
	for k, _ := range m {
		vec = append(vec, k)
	}
	sort.Strings(vec)
	return
}

func Pf(w io.Writer, f string, args ...any) {
	fmt.Fprintf(w, f+"\n", args...)
}
