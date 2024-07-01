package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"

	OWL "github.com/strickyak/ABhL"
)

var IPL = flag.String("ipl", "", "filename of bytes for Initial Program Load")
var MAX = flag.Int("max", 0, "Max number of steps to execute, after IPL (nonpositive means MaxInt)")

type ReadArgsWriteExit struct {
	args []byte
}

func NewReadArgsWriteExit() *ReadArgsWriteExit {
	var args []byte
	for _, a := range flag.Args() {
		args = append(args, []byte(a)...) // append bytes from the arg
		args = append(args, 0)            // terminated by a 0 byte
	}
	return &ReadArgsWriteExit{args: args}
}

func (rawe *ReadArgsWriteExit) Read() byte {
	if rawe.args != nil {
		z := rawe.args[0]
		rawe.args = rawe.args[1:]
		return z
	} else {
		return 0 // return 0's after args are exhausted
	}
}

func (rawe *ReadArgsWriteExit) Write(status byte) {
	log.Printf("ReadArgsWriteExit: EXIT $%02x", status)
	os.Exit(int(status))
}

type Terminal struct {
	r io.Reader
	w io.Writer
}

func (term *Terminal) Read() byte {
	bb := []byte{0}
	n, err := term.r.Read(bb)
	if err != nil || n != 1 {
		log.Fatalf("FATAL: Terminal stopping on bad Read (%d; %v)", n, err)
	}
	return bb[0]
}

func (term *Terminal) Write(x byte) {
	bb := []byte{x}
	n, err := term.w.Write(bb)
	if err != nil || n != 1 {
		log.Fatalf("FATAL: Terminal stopping on bad Write (%d; %v)", n, err)
	}
}

func main() {
	log.SetFlags(0) // dont need time and date
	flag.Parse()

	vec, err := ioutil.ReadFile(*IPL)
	if err != nil {
		log.Fatalf("FATAL: Cannot read IPL file %q: %v", *IPL, err)
	}

	term := &Terminal{
		r: os.Stdin,
		w: os.Stdout,
	}
	rawe := NewReadArgsWriteExit()
	vm := &OWL.Vm{
		F: term,
		G: rawe,
	}

	vm.IPL(vec)

	max := *MAX
	if max < 1 {
		const max_uint = ^(uint(0))
		const max_int = int(max_uint >> 1)
		max = max_int
	}
	ok := vm.Steps(max)

	if ok {
		log.Printf("owl-emu: Stopped after the max %d steps", *MAX)
	} else {
		log.Printf("owl-emu: Stopped before reaching the max steps")
	}
}
