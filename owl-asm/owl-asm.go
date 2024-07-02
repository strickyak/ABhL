package main

import (
	"flag"
	"fmt"
	"log"

	OWL "github.com/strickyak/ABhL"
)

var O = flag.String("o", "", "write IPL to this file")

func main() {
	log.SetFlags(0)
	flag.Parse()

	var lines []string
	var wheres []string
	for _, arg := range flag.Args() {
		slurp := OWL.SlurpTextFile(arg)
		lines = append(lines, slurp...)
		for i := 1; i <= len(slurp); i++ {
			wheres = append(wheres, fmt.Sprintf("%s:%d", arg, i))
		}
	}

	mod := OWL.ParseLines(lines, wheres)
	OWL.MacroPassOne(mod)
	OWL.MacroPassTwo(mod)
	OWL.PassOne(mod)
	OWL.PassTwo(mod)
	OWL.PassThree(mod)

	if *O != "" {
		OWL.WriteIPL(mod, *O)
	}
}
