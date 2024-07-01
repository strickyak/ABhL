package ABhL // pronounced "owl"

import (
	"fmt"
	"testing"
)

func Repr(obj any) string {
	return fmt.Sprintf("%#v", obj)
}

func Test1(t *testing.T) {
	for _, it := range []struct{ line, want string }{
		{
			"abhc",
			`&ABhL.Row{label:"abhc", opcode:"", args:[]string{""}, comment:"", instr:(*ABhL.Instr)(nil), length:0x0, addrX:(*ABhL.Expr)(nil), addr:0x0, final:false}`,
		},
		{
			"abhc: ; foo the bar",
			`&ABhL.Row{label:"abhc", opcode:"", args:[]string{""}, comment:"; foo the bar", instr:(*ABhL.Instr)(nil), length:0x0, addrX:(*ABhL.Expr)(nil), addr:0x0, final:false}`,
		},
		{
			"abhc LDA #90, y ; remark",
			`&ABhL.Row{label:"abhc", opcode:"LDA", args:[]string{"#90", " y "}, comment:"; remark", instr:(*ABhL.Instr)(nil), length:0x0, addrX:(*ABhL.Expr)(nil), addr:0x0, final:false}`,
		},
		{
			"abhc: bcd one,two,three;eight,nine,ten",
			`&ABhL.Row{label:"abhc", opcode:"bcd", args:[]string{"one", "two", "three"}, comment:";eight,nine,ten", instr:(*ABhL.Instr)(nil), length:0x0, addrX:(*ABhL.Expr)(nil), addr:0x0, final:false}`,
		},
	} {
		got := Repr(ParseLine(it.line))
		if got != it.want {
			t.Logf("L: %s", it.line)
			t.Logf("G: %s", got)
			t.Logf("W: %s", it.want)
			t.Errorf("G != W")
		}
	}
}
