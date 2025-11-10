package xexpect

import (
	"fmt"
	"io"
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

type XExpect struct {
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
	waitv    bool
}

type action struct {
	expect     string
	send       string
	isContinue bool // 匹配成功后是否继续匹配
	isReg      bool // 是否为正则
}

func NewXExpect() *XExpect {
	return &XExpect{
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
		waitv:    false,
	}
}

func (sf *XExpect) SetTimeout(second int) {
	if sf.step != stepNew {
		sf.errorf("SetTimeout() must be called before Run()")
	}

	if second < 1 {
		sf.errorf("timeout(second) must be greater than 0 ")
	}

	sf.timeout = second
}

func (sf *XExpect) Run(args []string) {
	if sf.step != stepNew {
		sf.errorf("Run() can be called only once")
	}
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
		    io.CopyBuffer(sf.ptmx, os.Stdin,sf.sbf)
		}()
	} else {
		go func() {
		    io.CopyBuffer(sf.ptmx, sf.term,sf.sbf)
		}()
	}
	// timeout
	time.AfterFunc(time.Second*time.Duration(sf.timeout), func() {
		if sf.step > stepExpect {
			return
		}
		sf.close()
		sf.errorf("timeout exit")
	})
}
func (sf *XExpect) saveVar(cutLen int,mlen int){
    if !sf.waitv {
        return
    }
    vx:=cutLen-mlen;
    vl:=len(sf.vbf)-sf.vlen;
    if vx>vl{
        vx=vl;
    }
    copy(sf.vbf[sf.vlen:],sf.buf[:vx])
    sf.vlen+=vx
}
func (sf *XExpect) cutBuf(dataLen int,cutLen int,mlen int){
    if cutLen<0{
        return
    }
    sf.saveVar(cutLen,mlen)
    copy(sf.buf, sf.buf[cutLen:])
    sf.start = dataLen-cutLen
}
func (sf *XExpect) streamFind(dst io.Writer, src io.Reader,matchb[]byte) (written int64, err error) {
    mlen:=len(matchb)
	for {
		nr, er := src.Read(sf.buf[sf.start:])
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
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
        idx := bytes.Index(sf.buf,matchb)
        if idx>-1 {
            sf.cutBuf(sf.start + nr,idx+mlen, mlen)
            return written, err
        }else{
            sf.cutBuf(sf.start + nr,sf.start+nr-mlen+1,0)
        }
	}
    return written, err
}
func (sf *XExpect) Matchs(rule [][]string) (int, string) {
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
		if err != nil {
			break
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
						if err != nil {
							sf.errorf("send error: %v", err)
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
func (sf *XExpect) TermUtil(wstr string) {
	if sf.term == nil {
		sf.streamFind(os.Stdout, sf.ptmx,[]byte(wstr))
	} else {
	    sf.streamFind(sf.term, sf.ptmx,[]byte(wstr))
	}
}
func (sf *XExpect) ValHex() string{
    return hex.EncodeToString(sf.vbf[0 : sf.vlen])
}
func (sf *XExpect) ValRaw() string{
    return string(sf.vbf[0 : sf.vlen])
}
func (sf *XExpect) ReadUtil(wstr string) string {
    sf.waitv=true
    sf.vlen=0
	if sf.term == nil {
		sf.streamFind(os.Stdout, sf.ptmx,[]byte(wstr))
	} else {
	    sf.streamFind(sf.term, sf.ptmx,[]byte(wstr))
	}
    sf.waitv=false
    x:=string(sf.vbf[0 : sf.vlen])
    var ret string
    fmt.Sscanf(x,"%s",&ret);
    return ret
}
func (sf *XExpect) Term() {
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

func (sf *XExpect) Exit() {
	if sf.step == stepExit {
		sf.errorf("Exit() does not allow repeated calls")
	}
	sf.step = stepExit

	sf.close()
}

func (sf *XExpect) close() {
	if sf.term != nil {
		sf.term.Close()
	}
	if sf.ptmx != nil {
		sf.ptmx.Close()
	}
}

func (sf *XExpect) regMatch(text string, reg string) string {
	regex, err := regexp.Compile(reg)
	if err != nil {
		sf.errorf("reg (%s) error: %v", reg, err)
	}

	return regex.FindString(text)
}

func (sf *XExpect) parseRule(rule [][]string) []*action {
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

func (sf *XExpect) onSizeChange(p *pty.Pty) func(uint16, uint16) {
	return func(cols, rows uint16) {
		size := &pty.WinSize{
			Cols: cols,
			Rows: rows,
		}
		p.SetSize(size)
	}
}

func (sf *XExpect) errorf(format string, vals ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", vals...)
	sf.close()
	os.Exit(1)
}
