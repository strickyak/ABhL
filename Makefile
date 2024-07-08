all: run-hello

hello.cb.genowl: hello.cb
	go run cflat/cflat.go $< 2> ,compile.err

hello.ipl: hello.cb.genowl
	go run owl-asm/owl-asm.go -o $@ $< lib1.owl > hello.listing 2> ,assemble.err

run-hello: hello.ipl
	go run owl-emu/owl-emu.go -ipl $< 2> ,emu.err
