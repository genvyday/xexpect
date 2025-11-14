package main

import (
	"fmt"
	"strconv"
	"os"
	"runtime"
	"encoding/base64"

	"golang.org/x/term"
	"github.com/dop251/goja"
	"tmhelper/tmhelper"
)

type JS struct {
	vm *goja.Runtime
	xe *tmhelper.TMHelper
	cptkey  []byte
}

func NewJS() *JS {
	return &JS{
		vm: goja.New(),
		xe: tmhelper.NewTMHelper(),
		cptkey: nil,
	}
}

func (sf *JS) Run(jsCode string) goja.Value {
    sf.vm.Set("tmh", sf)
	v, err := sf.vm.RunString(jsCode)
	if err != nil {
		panic(err)
	}
    return v;
}
func (sf *JS) CptKey(value goja.FunctionCall) goja.Value {
    ekey:=value.Argument(0).String()
    chk:=value.Argument(1).String()
    enc:=tmhelper.EncText("123",ekey)
    valid:=enc==chk;
    if(valid){
        sf.cptkey=tmhelper.GenKey([]byte(ekey),32)
    }
	return sf.vm.ToValue(valid)
}
func (sf *JS) Ok(value goja.FunctionCall) goja.Value {
	return sf.vm.ToValue(sf.xe.Ok())
}
func (sf *JS) Dec(value goja.FunctionCall) goja.Value {
    str := value.Argument(0)
    if sf.cptkey==nil||len(sf.cptkey)==0{
        return str
    }
    encData,_:=base64.RawURLEncoding.DecodeString(str.String())
	plain:=string(tmhelper.AesDec(encData,sf.cptkey))
	return sf.vm.ToValue(plain)
}

func (sf *JS) Goos(value goja.FunctionCall) goja.Value {
    return sf.vm.ToValue(runtime.GOOS)
}
func (sf *JS) Enc(value goja.FunctionCall) goja.Value {
    str := value.Argument(0)
    if sf.cptkey==nil||len(sf.cptkey)==0{
        return str
    }
	encData:=tmhelper.AesEnc([]byte(str.String()),sf.cptkey)
	ret:=base64.RawURLEncoding.EncodeToString(encData)
	return sf.vm.ToValue(ret)
}
// func(args []string)
func (sf *JS) Exec(value goja.FunctionCall) goja.Value {
	args := sf.formatArgs(value.Argument(0))
	fmt.Println(args)
	sf.xe.Run(args)
	return sf.vm.ToValue(nil)
}

func (sf *JS) SetTimeout(value goja.FunctionCall) goja.Value {
	str := value.Argument(0).String()
	sec,err := strconv.Atoi(str)
	if err != nil {
		sf.errorf("setTimeout error: must number")
	}
	sf.xe.SetTimeout(sec)
	return sf.vm.ToValue(nil)
}

// func (rule [][]string) map[string]any{"idx": idx, "str": str}
func (sf *JS) Matchs(value goja.FunctionCall) goja.Value {
	rule := sf.formatRule(value.Argument(0))
	idx, str := sf.xe.Matchs(rule)
	return sf.vm.ToValue(map[string]any{"idx": idx, "str": str})
}

func (sf *JS) Term(_ goja.FunctionCall) goja.Value {
	sf.xe.Term()
	return sf.vm.ToValue(nil)
}
func (sf *JS) Expect(call goja.FunctionCall) goja.Value {
	str := call.Argument(0)
	sf.xe.Expect(str.String())
	return str
}
func (sf *JS) ValRaw(call goja.FunctionCall) goja.Value {
    ret :=sf.xe.ValRaw()
    return sf.vm.ToValue(ret)
}
func (sf *JS) ValHex(call goja.FunctionCall) goja.Value {
    ret :=sf.xe.ValHex()
    return sf.vm.ToValue(ret)
}
func (sf *JS) ReadStr(call goja.FunctionCall) goja.Value {
	str := call.Argument(0)
	ret :=sf.xe.ReadStr(str.String())
	return sf.vm.ToValue(ret)
}
func (sf *JS) WaitDone(call goja.FunctionCall) goja.Value {
	arg:=call.Argument(0)
	str:=""
	if arg!=goja.Undefined(){
	    str=arg.String()
	}
    sf.xe.WaitRelayExit(str)
    return arg;
}
func (sf *JS) Exit(call goja.FunctionCall) goja.Value {
	sf.xe.Exit()
	return sf.vm.ToValue(nil)
}
func (sf *JS) Pwd(call goja.FunctionCall) goja.Value {
	str := call.Argument(0)
	fmt.Print(str.String())
	bpwd,_ := term.ReadPassword(int(os.Stdin.Fd()))
	return sf.vm.ToValue(string(bpwd))
}
func (sf *JS) Input(call goja.FunctionCall) goja.Value {
	str := sf.xe.ReadInput(call.Argument(0).String())
	return sf.vm.ToValue(str)
}
func (sf *JS) Println(call goja.FunctionCall) goja.Value {
	str := call.Argument(0)
	fmt.Println(str.String())
	return str
}
func (sf *JS) Print(call goja.FunctionCall) goja.Value {
	str := call.Argument(0)
	fmt.Print(str.String())
	return str
}
func (sf *JS) formatArgs(value goja.Value) []string {
	arrayInterface, ok := value.Export().([]interface{})
	if !ok {
		sf.errorf("run args error: must string array")
	}

	var args []string
	for _, item := range arrayInterface {
		str, ok := item.(string)
		if !ok {
			sf.errorf("run args error: must string array")
		}
		args = append(args, str)
	}

	return args
}

func (sf *JS) formatRule(value goja.Value) [][]string {
	var rule [][]string

	// 断言Value是一个数组
	arr1, ok := value.Export().([]interface{})
	if !ok {
		errorf("matchs args error: must two-dimensional string array")
	}

	// 遍历第一层数组
	for i := 0; i < len(arr1); i++ {
		arr2, ok := arr1[i].([]interface{})
		if !ok {
			errorf("matchs args error: must two-dimensional string array")
		}

		var innerArray []string

		// 遍历内层数组
		for _, elem := range arr2 {
			strVal, ok := elem.(string)
			if !ok {
				errorf("matchs args error: must two-dimensional string array")

			}
			innerArray = append(innerArray, strVal)
		}

		rule = append(rule, innerArray)
	}

	return rule
}

func (sf *JS) errorf(format string, vals ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", vals...)
	if sf.xe != nil {
		sf.xe.Exit()
	}
}
