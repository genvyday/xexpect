package tmhelper

import (
	"fmt"
	"io"
	"bufio"
	"os"
	"regexp"
	"strings"
	"time"
	"errors"
	"bytes"
    "encoding/hex"

	"github.com/iyzyi/aiopty/pty"
	"github.com/iyzyi/aiopty/term"
)

const (
	stepNew      = 0
	stepRun      = 1
	stepExpect   = 2
	stepInteract = 3
	stepExit     = 4

	matchLen = 32
)

type TMHelper struct {
	ptmx    *pty.Pty
	term    *term.Term
	timeout int // 总超时时间，秒

	step     int
	buf      []byte
	start    int
	matchLen int
	sbf      []byte
	vbf      []byte
	vlen     int
	key      []byte
	err      error
	timer    *time.Timer
}

type action struct {
	expect     string
	send       string
	isContinue bool // 匹配成功后是否继续匹配
	isReg      bool // 是否为正则
}

func NewTMHelper() *TMHelper {
	return &TMHelper{
		ptmx:     nil,
		term:     nil,
		timeout:  10,
		step:     stepNew,
		buf:      make([]byte, 1024),
		start:    0,
		matchLen: matchLen,
		sbf:      make([]byte, 1024),
		vbf:      make([]byte,1024),
		vlen:     0,
		err:      nil,
		timer:    nil,
	}
}

func (sf *TMHelper) SetTimeout(second int) {
	if sf.step != stepNew {
		sf.errorf("SetTimeout() must be called before Run()")
	}

	if second < 1 {
		sf.errorf("timeout(second) must be greater than 0 ")
	}

	sf.timeout = second
}

func (sf *TMHelper) Run(args []string) {
    sf.err = nil
    sf.start=0
	sf.vlen=  0
	sf.step = stepRun

	opt := &pty.Options{
		Path: args[0],
		Args: args,
		Dir:  "",
		Env:  nil,
		Size: &pty.WinSize{
			Cols: 120,
			Rows: 30,
		},
		Type: "",
	}
	p, err := pty.OpenWithOptions(opt)
	if err != nil {
		sf.errorf("Failed to create pty: %v", err)
	}
	//defer p.Close()
	sf.ptmx = p

	// terminal
	t, err := term.Open(os.Stdin, os.Stdout, sf.onSizeChange(p))
	if err == nil {
		sf.term = t
	}
	// 响应手动输入
	if sf.term == nil {
		go func() {
		    _,err=io.CopyBuffer(sf.ptmx, os.Stdin,sf.sbf)
		}()
	} else {
		go func() {
		    _,err=io.CopyBuffer(sf.ptmx, sf.term,sf.sbf)
		}()
	}
	// timeout
	sf.timer=time.AfterFunc(time.Second*time.Duration(sf.timeout), func() {
		if sf.step > stepExpect {
			return
		}
		sf.close()
		sf.errorf("timeout exit")
	})
}
func (sf *TMHelper) AesKey(key []byte){
    if len(key) !=0{
        sf.key=GenKey(key,32)
    }
}
func (sf *TMHelper) Enc(plain []byte)([] byte){
    return AesEnc(plain,sf.key)
}
func (sf *TMHelper) Dec(enc []byte)([] byte){
    return AesDec(enc,sf.key)
}
func (sf *TMHelper) saveVal(cutLen int,mlen int){
    vx:=cutLen-mlen;
    vl:=len(sf.vbf)-sf.vlen;
    if vx>vl{
        vx=vl;
    }
    copy(sf.vbf[sf.vlen:],sf.buf[:vx])
    sf.vlen+=vx
}
func (sf *TMHelper) cutBuf(dataLen int,cutLen int,mlen int,readVal bool){
    if cutLen<0{
        return
    }
    if readVal{
        sf.saveVal(cutLen,mlen)
    }
    copy(sf.buf, sf.buf[cutLen:])
    sf.start = dataLen-cutLen
}
func ReadStr(in io.Reader)string{
    inputReader := bufio.NewReader(in)
    str, _ := inputReader.ReadString('\n')
    return strings.Trim(str," \t\n\r")
}
func (sf *TMHelper) streamFind(dst io.Writer, src io.Reader,str string,readVal bool) (written int64, err error) {
    matchb:=[]byte(str)
    mlen:=len(matchb)
	for {
		nr, er := src.Read(sf.buf[sf.start:])
		sf.err=er
		if nr > 0 {
			nw, ew := dst.Write(sf.buf[sf.start : sf.start+nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
            idx := bytes.Index(sf.buf[:sf.start + nr],matchb)
            if idx>-1 {
                sf.cutBuf(sf.start + nr,idx+mlen, mlen,readVal)
                return written, err
            }else{
                sf.cutBuf(sf.start + nr,sf.start+nr-mlen+1,0,readVal)
            }
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
    return written, err
}
func (sf *TMHelper) Matchs(rule [][]string) (int, string) {
	if sf.step < stepRun {
		sf.errorf("Matchs() must be called after Run()")
	}
	if sf.step > stepExpect {
		sf.errorf("Matchs() must be called befor Exit() and Interact() ")
	}
	sf.step = stepExpect

	listAction := sf.parseRule(rule)

	isMatch := false
	matchStr := ""
	cutLen := 0

	for {
		n, err := sf.ptmx.Read(sf.buf[sf.start:])
		sf.err=err
		if err != nil {
		    sf.errorf("read error: %v", err)
		    return -1,""
		}
		fmt.Print(string(sf.buf[sf.start : sf.start+n])) // 显示pty输出
		if n > 0 {
			text := string(sf.buf[:sf.start+n])

			for i, act := range listAction {
				// 匹配
				isMatch = false
				matchStr = ""
				cutLen = 0
				if len(act.expect) == 0 { // 不匹配直接发送
					isMatch = true
					matchStr = ""
					cutLen = len(text)
				} else if act.isReg {
					matchStr = sf.regMatch(text, act.expect)
					if matchStr != "" {
						isMatch = true
					}
				} else {
					idx := strings.LastIndex(text, act.expect)
					if idx > -1 {
						isMatch = true
						cutLen = idx + len(act.expect)
					}
				}

				//抛弃已匹配字符串前面的字符串，避免重复匹配
				if cutLen > 0 {
					copy(sf.buf, sf.buf[cutLen:])
					sf.start = sf.start + n - cutLen
				}

				// 匹配成功，
				if isMatch {
					if len(act.send) > 0 {
						//fmt.Println(act.send, "end")
						_, err := sf.ptmx.Write([]byte(act.send))
						sf.err=err
						if err != nil {
							sf.errorf("send error: %v", err)
							return -1,""
						}
					}
					if act.isContinue {
						break
					} else {
						return i, matchStr
					}
				}
			}
		}

		//
		if sf.start+n > sf.matchLen {
			copy(sf.buf, sf.buf[sf.matchLen:])
			sf.start = sf.start + n - sf.matchLen
		}
	}

	return -1, ""
}
func (sf *TMHelper) Expect(wstr string) {
	if sf.term == nil {
		sf.streamFind(os.Stdout, sf.ptmx,wstr,false)
	} else {
	    sf.streamFind(sf.term, sf.ptmx,wstr,false)
	}
}
func (sf *TMHelper) ValHex() string{
    return hex.EncodeToString(sf.vbf[0 : sf.vlen])
}
func (sf *TMHelper) ValRaw() string{
    return string(sf.vbf[0 : sf.vlen])
}
func formal(x string) string{
    s,ret,sp:="","",""
    reader := strings.NewReader(x)
    for reader.Len()>0{
        ns,_:= fmt.Fscanf(reader,"%s",&s);
        if ns!=0 {
            ret=ret+sp+s
            sp=" "
        }
    }
    return ret
}
func (sf *TMHelper) ReadStr(wstr string) string {
    sf.vlen=0
	if sf.term == nil {
		sf.streamFind(os.Stdout, sf.ptmx,wstr,true)
	} else {
	    sf.streamFind(sf.term, sf.ptmx,wstr,true)
	}
    return formal(string(sf.vbf[0 : sf.vlen]))
}
func (sf *TMHelper) Ok() bool{
    return sf.step != stepExit&&sf.err==nil;
}
func (sf *TMHelper) Error() error{
    return sf.err
}
func (sf *TMHelper) Term() {
	if sf.step < stepRun {
		sf.errorf("Matchs() must be called after Run()")
	}
	if sf.step == stepInteract {
		sf.errorf("Interact() does not allow repeated calls")
	}
	if sf.step == stepExit {
		sf.errorf("Interact() can not called after Exit()")
	}
	sf.step = stepInteract
	defer sf.close()

	if sf.term == nil {
		io.Copy(os.Stdout, sf.ptmx)
	} else {
		io.Copy(sf.term, sf.ptmx)
	}
}

func (sf *TMHelper) Exit() {
	if sf.step != stepExit {
	    sf.close()
	}
	sf.step = stepExit
}

func (sf *TMHelper) close() {
	if sf.term != nil {
		sf.term.Close()
		sf.term=nil
	}
	if sf.ptmx != nil {
		sf.ptmx.Close()
		sf.ptmx=nil
	}
    if sf.timer != nil{
        sf.timer.Stop()
        sf.timer=nil
    }
}

func (sf *TMHelper) regMatch(text string, reg string) string {
	regex, err := regexp.Compile(reg)
	if err != nil {
		sf.errorf("reg (%s) error: %v", reg, err)
	}

	return regex.FindString(text)
}

func (sf *TMHelper) parseRule(rule [][]string) []*action {
	out := make([]*action, 0, len(rule))

	numC := 0
	for _, v := range rule {
		if len(v) < 2 {
			sf.errorf("rule error: The actions must contain at least two elements: match and send")
		}
		act := &action{
			expect:     v[0],
			send:       v[1],
			isContinue: false,
			isReg:      false,
		}
		for i := range v[2:] {
			switch v[2+i] {
			case "C":
				act.isContinue = true
				numC++
			case "E":
				act.isReg = true
			default:
				sf.errorf("rule error: flag = '%s', want 'C' or 'E'", v[2+i])
			}
		}
		out = append(out, act)
	}

	if numC == len(rule) {
		sf.errorf("rule error: It's not allowed for all actions to set the C flag")
	}

	return out
}

func (sf *TMHelper) onSizeChange(p *pty.Pty) func(uint16, uint16) {
	return func(cols, rows uint16) {
		size := &pty.WinSize{
			Cols: cols,
			Rows: rows,
		}
		p.SetSize(size)
	}
}

func (sf *TMHelper) errorf(format string, vals ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", vals...)
}
