package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"bufio"
	"strings"
	"encoding/base64"
	"tmhelper/tmhelper"
)
const PWDENV ="TMHPWD"
func main() {
	jsPath := flag.String("f", "", "file, 运行tmhelper js文件")
	jxPath := flag.String("x", "", "file, 运行tmhelper js文件")
	jsCode := flag.String("c", "", "code, 运行tmhelper js代码")
	encText := flag.String("e", "", "encrypt,加密文本")
	decText := flag.String("d", "", "decrypt,解密文本")
	flag.Parse()
    pwd:=os.Getenv(PWDENV)
	code := []byte{}
	if *jsPath != "" {
		code = readFromFile(*jsPath)
	} else if *jxPath != "" {
     	code = readFromFile(*jxPath)
     	fmt.Print("Please enter password decrypt key: ")
     	inputReader := bufio.NewReader(os.Stdin)
     	pwd, _ = inputReader.ReadString('\n')
     	pwd=strings.Trim(pwd," \t\n\r")
    } else if *jsCode != "" {
		code = []byte(*jsCode)
	} else if *encText != "" {
		encryptText(*encText)
		return
	}  else if *decText != "" {
      	decryptText(*decText)
      	return
    }else {
		code = readFromStdin()
	}
	js := NewJS()
	js.Run(string(code),pwd)
}

func readFileCode(jsPath string) (b []byte) {
    b = readFromFile(jsPath)
	return b
}

func readFromFile(jsPath string) []byte {
	b, err := os.ReadFile(jsPath)
	if err != nil {
		errorf("read js file error: %v", err)
	}

	return b
}

func encryptText(plain string) {
    pwd:=os.Getenv(PWDENV)
    key:=tmhelper.GenKeyX(pwd,32)
    encd:=tmhelper.AesEnc([]byte(plain),key)
    encText:=base64.RawURLEncoding.EncodeToString(encd);
    fmt.Println(encText)
}
func decryptText(encstr string) {
    pwd:=os.Getenv(PWDENV)
    key:=tmhelper.GenKeyX(pwd,32)
    encData,_:=base64.RawURLEncoding.DecodeString(encstr)
    plain:=string(tmhelper.AesDec(encData,key))
    fmt.Println(plain)
}

func readFromStdin() []byte {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		errorf("read stdin error: %v", err)
	}
	return b
}

func errorf(format string, vals ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", vals...)
	os.Exit(1)
}
