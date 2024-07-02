package main

import (
	"flag"
	"log"

	OWL "github.com/strickyak/ABhL"
)

var O = flag.String("o", "", "write IPL to this file")

func main() {
	log.SetFlags(0)
	flag.Parse()

	var lines []string
	for _, arg := range flag.Args() {
		slurp := OWL.SlurpTextFile(arg)
		lines = append(lines, slurp...)
	}
	mod := OWL.ParseLines(lines)
	OWL.PassOne(mod)
	OWL.PassTwo(mod)
	OWL.PassThree(mod)

	// OWL.LogRows(mod)
	// OWL.LogRam(mod)

	if *O != "" {
		OWL.WriteIPL(mod, *O)
	}
}
