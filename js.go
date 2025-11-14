package main

import (
	"fmt"
	"strconv"
	"os"
	"encoding/base64"

	"golang.org/x/term"
	"github.com/dop251/goja"
	"tmhelper/tmhelper"
)

type JS struct {
	vm *goja.Runtime
	xe *tmhelper.TMHelper
	pwd string
}

func NewJS() *JS {
	return &JS{
		vm: goja.New(),
		xe: tmhelper.NewTMHelper(),
		pwd: "",
	}
}

func (sf *JS) Run(jsCode string) {
	sf.vm.Set("tmh_run", sf.tmh_run)                // func(args []string)
	sf.vm.Set("tmh_setTimeout", sf.tmh_setTimeout) // func(second int)
	sf.vm.Set("tmh_matchs", sf.tmh_matchs)          // func (rule [][]string) map[string]any{"idx": idx, "str": str}
	sf.vm.Set("tmh_term", sf.tmh_term)              // func()
	sf.vm.Set("tmh_expect", sf.tmh_expect)      // func()
	sf.vm.Set("tmh_readStr", sf.tmh_readStr)      // func()
	sf.vm.Set("tmh_valHex", sf.tmh_valHex)          // func()
	sf.vm.Set("tmh_cptKey", sf.tmh_cptKey)          // func()
	sf.vm.Set("tmh_dec", sf.tmh_dec)          // func()
	sf.vm.Set("tmh_enc", sf.tmh_enc)          // func()
	sf.vm.Set("tmh_ok", sf.tmh_ok)          // func()
	sf.vm.Set("tmh_input", sf.tmh_input)          // func()
	sf.vm.Set("tmh_pwd", sf.tmh_pwd)          // func()
	sf.vm.Set("tmh_valRaw", sf.tmh_valRaw)          // func()
	sf.vm.Set("tmh_exit", sf.tmh_exit)              // func()
	sf.vm.Set("tmh_println", sf.tmh_println)        // func(msg string)
	sf.vm.Set("tmh_print", sf.tmh_print)        // func(msg string)
	_, err := sf.vm.RunString(jsCode)
	if err != nil {
		panic(err)
	}
}
func (sf *JS) putKey(key string){
    sf.pwd=key
	sf.xe.AesKey([]byte(key))
}
func (sf *JS) tmh_cptKey(value goja.FunctionCall) goja.Value {
    ekey:=value.Argument(0).String()
    chk:=value.Argument(1).String()
    enc:=tmhelper.EncText("123",ekey)
    valid:=enc==chk;
    if(valid){
        sf.putKey(ekey)
    }
	return sf.vm.ToValue(valid)
}
func (sf *JS) tmh_ok(value goja.FunctionCall) goja.Value {
	return sf.vm.ToValue(sf.xe.Ok())
}
func (sf *JS) tmh_dec(value goja.FunctionCall) goja.Value {
    str := value.Argument(0)
    if len(sf.pwd)==0{
        return str
    }
    encData,_:=base64.RawURLEncoding.DecodeString(str.String())
	plain:=string(sf.xe.Dec(encData))
	return sf.vm.ToValue(plain)
}

func (sf *JS) tmh_enc(value goja.FunctionCall) goja.Value {
    str := value.Argument(0)
    if len(sf.pwd)==0{
        return str
    }
	encData:=sf.xe.Enc([]byte(str.String()))
	ret:=base64.RawURLEncoding.EncodeToString(encData)
	return sf.vm.ToValue(ret)
}
// func(args []string)
func (sf *JS) tmh_run(value goja.FunctionCall) goja.Value {
	args := sf.formatArgs(value.Argument(0))
	fmt.Println(args)
	sf.xe.Run(args)
	return sf.vm.ToValue(nil)
}

func (sf *JS) tmh_setTimeout(value goja.FunctionCall) goja.Value {
	str := value.Argument(0).String()
	sec,err := strconv.Atoi(str)
	if err != nil {
		sf.errorf("setTimeout error: must number")
	}
	sf.xe.SetTimeout(sec)
	return sf.vm.ToValue(nil)
}

// func (rule [][]string) map[string]any{"idx": idx, "str": str}
func (sf *JS) tmh_matchs(value goja.FunctionCall) goja.Value {
	rule := sf.formatRule(value.Argument(0))
	idx, str := sf.xe.Matchs(rule)
	return sf.vm.ToValue(map[string]any{"idx": idx, "str": str})
}

func (sf *JS) tmh_term(_ goja.FunctionCall) goja.Value {
	sf.xe.Term()
	return sf.vm.ToValue(nil)
}
func (sf *JS) tmh_expect(call goja.FunctionCall) goja.Value {
	str := call.Argument(0)
	sf.xe.Expect(str.String())
	return str
}
func (sf *JS) tmh_valRaw(call goja.FunctionCall) goja.Value {
    ret :=sf.xe.ValRaw()
    return sf.vm.ToValue(ret)
}
func (sf *JS) tmh_valHex(call goja.FunctionCall) goja.Value {
    ret :=sf.xe.ValHex()
    return sf.vm.ToValue(ret)
}
func (sf *JS) tmh_readStr(call goja.FunctionCall) goja.Value {
	str := call.Argument(0)
	ret :=sf.xe.ReadStr(str.String())
	return sf.vm.ToValue(ret)
}
func (sf *JS) tmh_exit(_ goja.FunctionCall) goja.Value {
	sf.xe.Exit()
	return sf.vm.ToValue(nil)
}
func (sf *JS) tmh_pwd(call goja.FunctionCall) goja.Value {
	str := call.Argument(0)
	fmt.Print(str.String())
	bpwd,_ := term.ReadPassword(int(os.Stdin.Fd()))
	return sf.vm.ToValue(string(bpwd))
}
func (sf *JS) tmh_input(call goja.FunctionCall) goja.Value {
	str := sf.xe.ReadInput(call.Argument(0).String())
	return sf.vm.ToValue(str)
}
func (sf *JS) tmh_println(call goja.FunctionCall) goja.Value {
	str := call.Argument(0)
	fmt.Println(str.String())
	return str
}
func (sf *JS) tmh_print(call goja.FunctionCall) goja.Value {
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
