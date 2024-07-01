# ABhL
Assembler and Emulator for a proposed TTL CPU: ABhL (pronounced "owl")

## quick hint

To Assemble:

```
echo 'start:  setb 100 ; some comments' | go run owl-asm/owl-asm.go -o a.out /dev/stdin
```

To Run:

```
go run owl-emu/owl-emu.go -ipl a.out
```
