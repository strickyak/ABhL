# ABhL 8-bit TTL CPU
Assembler and Emulator for a proposed TTL CPU: ABhL (pronounced "owl")

## quick hint

To Assemble:

```
go run owl-asm/owl-asm.go  -o a.out  brett.owl lib1.owl ... other .owl files ...
```

The output file `a.out` contains bytes to be fed
to the ABhL machine during Initial Program Load.
After loading everything into initial memory,
it jumps to the label named `start`.

To Run:

```
go run owl-emu/owl-emu.go  -ipl a.out 2>_log
```

Lots of debugging output goes to stderr.
That command captured it and put it in the file `_log`.
Examine that file to debug.

In the emulator, port E is undefined.

Port F reads from stdin and writes to stdout.

Reading from Port G reads the emulator's command line arguments,
with the words '\0'-terminated, and reading '\0's
when exhausted.

Writing to Port G exits the emulator.
The byte written is the exit status.

Also executing a $00 instruction will exit the emulator.
