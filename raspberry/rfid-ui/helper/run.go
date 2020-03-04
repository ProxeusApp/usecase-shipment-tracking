package helper

import (
	"log"
	"os"
	"os/exec"
	"io"
	"bytes"
	"strings"
	"regexp"
	"sync"
	"runtime/debug"
	"context"
)

type SytraxPhytonHandler struct {
	ctx context.Context
	cancel context.CancelFunc
	cmd         *exec.Cmd
	lastNumber  string
	cmdInput    map[string][]string
	dLock       sync.Mutex
	cmdLock     sync.Mutex
	statusInput map[string]*Status3
	sLock       sync.Mutex
	wg          sync.WaitGroup
	interrupted bool
	tableStart bool
}

type Status3 struct {
	Status     int
	ScanActive bool
	Table      [][]string
}

func New() *SytraxPhytonHandler {
	return &SytraxPhytonHandler{cmdInput: make(map[string][]string), statusInput: make(map[string]*Status3)}
}

// CapturingPassThroughWriter is a writer that remembers
// data written to it and passes it to w
type CapturingPassThroughWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
	w   io.Writer
}

// NewCapturingPassThroughWriter creates new CapturingPassThroughWriter
func NewCapturingPassThroughWriter(w io.Writer) *CapturingPassThroughWriter {
	return &CapturingPassThroughWriter{
		w: w,
	}
}

func (w *CapturingPassThroughWriter) Write(d []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(d)
	return w.w.Write(d)
}

// Bytes returns bytes written to the writer
func (w *CapturingPassThroughWriter) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Bytes()
}

func (w *CapturingPassThroughWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	//if w.buf.Len() > 0 {
	//	log.Println(w.buf.String())
	//}
	return w.buf.String()
}
func (w *CapturingPassThroughWriter) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Reset()
}

func (me *SytraxPhytonHandler) Run(num string, consId string) error {
	me.cmdLock.Lock()
	defer me.cmdLock.Unlock()
	if num != "" {
		me.kill()
		me.lastNumber = num

		if me.cmd == nil {
			//me.ctx, me.cancel = context.WithTimeout(context.Background(), time.Second*3)
			//me.cmd = exec.CommandContext(me.ctx,"python", "sytrax1.py")
			me.cmd = exec.Command("python", "sytrax1.py")

			me.cmd.Dir = "/home/pi/MFRC522-python"
			me.interrupted = false
			me.tableStart = false
		}

		mstdout := NewCapturingPassThroughWriter(os.Stdout)
		mstderr := NewCapturingPassThroughWriter(os.Stderr)

		//var stderr io.ReadCloser
		//stdout, err := me.cmd.StdoutPipe()
		//if err != nil {
		//	log.Println("err..", err)
		//	return
		//}
		////me.wg.Add(3)
		//stderr, err = me.cmd.StderrPipe()
		//if err != nil {
		//	log.Println("err1..", err)
		//	return
		//}
		//me.cmd.Stdout = mstdout
		//me.cmd.Stderr = mstderr

		stdin, err := me.cmd.StdinPipe()
		if err != nil {
			log.Println("er..", err)
			return err
		}

		var stdout io.ReadCloser
		var stderr io.ReadCloser
		stdout, err = me.cmd.StdoutPipe()
		if err != nil {
			log.Println("err..", err)
			return err
		}
		//me.wg.Add(3)
		stderr, err = me.cmd.StderrPipe()
		if err != nil {
			log.Println("err1..", err)
			return err
		}

		err = me.cmd.Start() //non blocking
		if err != nil {
			log.Println("err2..", err)
			return err
		}
		check := func() {
			//log.Println("checking...")
			stoutstr := mstdout.String()
			//log.Println(stoutstr)
			//log.Println("-------------------------")
			me.checkAndCleanKownWarnings(mstderr, mstdout)
			if len(mstderr.String()) > 0 {
				me.setStatus(me.lastNumber, Status3{Status: 0, ScanActive: false})
				log.Println(mstderr.buf.String())
				mstderr.Reset()
				return
			} else if len(stoutstr) > 0 {
				if strings.Contains(stoutstr, "lease choose and pr") {
					mstdout.Reset()
					//log.Println("entering number", me.lastNumber)
					_, err = io.WriteString(stdin, me.lastNumber+"\n")
				} else if strings.Contains(stoutstr, "lease enter the consignment ID and press enter:") {
					mstdout.Reset()
					//log.Println("found Please enter the consignment ID and press enter:...")
					me.setStatus(me.lastNumber, Status3{Status: 1, ScanActive: false})
					_, err = io.WriteString(stdin, consId+"\n")
					if err != nil {
						return
					}
				} else if strings.Contains(stoutstr, "lease enter your name for the sign-off") {
					mstdout.Reset()
					me.setStatus(me.lastNumber, Status3{Status: 1, ScanActive: false})
					_, err = io.WriteString(stdin, consId+"\n")
					if err != nil {
						return
					}
				} else if strings.Contains(stoutstr, "press y to continue") {
					mstdout.Reset()
					//log.Println("found press y to continue...")
					_, err = io.WriteString(stdin, "y\n")
					if err != nil {
						return
					}
				} else if strings.Contains(stoutstr, "lease hold the") {
					//log.Println("found Please hold the RFID card near the reader...")
					if me.lastNumber == "3" {
						me.setStatus(me.lastNumber, Status3{Status: 3, ScanActive: true})
					} else {
						me.setStatus(me.lastNumber, Status3{Status: 0, ScanActive: true})
					}
					mstdout.Reset()
				} else if strings.Contains(stoutstr, "lease place th") {
					//log.Println("found Please place the tag on the reader...")
					me.setStatus(me.lastNumber, Status3{Status: 2, ScanActive: true})
					mstdout.Reset()
				} else if addr := FindAddress(stoutstr); addr != "" {
					//log.Println("found address!", addr)
					me.setStatus(me.lastNumber, Status3{Status: 0, ScanActive: false})
					me.push(me.lastNumber, addr)
					mstdout.Reset()
					//if me.lastNumber == "1" {
					//	me.kill()
					//}
				} else if table := ReadTableData(stoutstr); len(table) > 0 {
					me.tableStart = true
				}

				if me.tableStart && strings.Contains(stoutstr, "Welcome to the Blockfactory Tracking System") {
					if table := ReadTableData(stoutstr); len(table) > 0{
						me.tableStart = false
						//log.Println("found the table", addr)
						me.setStatus(me.lastNumber, Status3{Status: 0, ScanActive: false, Table: table})
						mstdout.Reset()
					}
				}
			}
			//if me.interrupted {
			//	break
			//}
			//time.Sleep(time.Millisecond*30)
			//}
			//stdin.Close()
			//stdout.Close()
			//stderr.Close()
			//log.Println("stop main routine")
			//me.setStatus(me.lastNumber, Status3{Status:0, ScanActive:false})
			//me.wg.Done()
			//}()
		}

		go func() {
			//defer me.cancel() // The cancel should be deferred so resources are cleaned up
			//log.Println("mstdout start...")
			myPipe(mstdout, stdout, check)
			stdout.Close()
			//log.Println("mstdout done...")
		}()
		go func() {
			//defer me.cancel() // The cancel should be deferred so resources are cleaned up
			//log.Println("mstderr start...")
			myPipe(mstderr, stderr, check)
			stderr.Close()
			//log.Println("mstderr done...")

		}()

		//go func() {
		//	log.Println("mstdout start...")
		//	io.Copy(mstdout, stdout)
		//	log.Println("mstdout done...")
		//}()
		//go func() {
		//	log.Println("mstderr start...")
		//	io.Copy(mstderr, stderr)
		//	//me.wg.Done()
		//	log.Println("mstderr done...")
		//}()

	}
	return nil
}

func myPipe(dst io.Writer, src io.Reader, clb func()){
	//defer me.cancel() // The cancel should be deferred so resources are cleaned up
	p := make([]byte, 2200)
	for {
		n, err := src.Read(p)
		if err != nil {
			log.Println("reader.Read err1..", err)
			break
		}
		if n > 0 {
			if n, err := dst.Write(p[:n]); err != nil {
				log.Println("error", n, err)
				break
			}
			clb()
		}
	}
}

func (me *SytraxPhytonHandler) checkAndCleanKownWarnings(mstderr, mstdout *CapturingPassThroughWriter) bool {
	if mstderr.buf.Len() > 0 && strings.Contains(mstderr.String(), "GPIO") {
		//log.Println("clean error...")
		mstderr.Reset()
		debug.FreeOSMemory()
		return true
	}
	return false
}

func (me *SytraxPhytonHandler) push(num, d string) {
	me.dLock.Lock()
	arr := me.cmdInput[num]
	if len(arr) > 0 {
		me.cmdInput[num] = append(arr, d)
	} else {
		me.cmdInput[num] = []string{d}
	}
	me.dLock.Unlock()
}

func (me *SytraxPhytonHandler) Get(num string) []string {
	me.dLock.Lock()
	arr := me.cmdInput[num]
	var c []string
	if arr != nil {
		c = make([]string, 0)
		if len(arr) > 0 {
			for i:= len(arr)-1; i >= 0; i--{
				c = append(c, arr[i])
			}
		}
	} else {
		c = make([]string, 0)
	}

	me.dLock.Unlock()
	return c
}

func (me *SytraxPhytonHandler) Status(num string) interface{} {
	me.sLock.Lock()
	defer me.sLock.Unlock()
	return me.statusInput[num]
}

func (me *SytraxPhytonHandler) setStatus(num string, n Status3) {
	me.sLock.Lock()
	existing := me.statusInput[num]
	if existing != nil {
		if len(n.Table)>0{
			existing.Table = n.Table
		}
		existing.ScanActive = n.ScanActive
		existing.Status = n.Status
	}else{
		me.statusInput[num] = &n
	}
	me.sLock.Unlock()
}

func (me *SytraxPhytonHandler) resetStatus() {
	me.sLock.Lock()
	me.statusInput = make(map[string]*Status3)
	me.sLock.Unlock()
}

func (me *SytraxPhytonHandler) Kill() error {
	me.cmdLock.Lock()
	defer me.cmdLock.Unlock()
	me.kill()
	return nil
}

func (me *SytraxPhytonHandler) kill() {
	me.interrupted = true
	if me.cmd != nil && me.cmd.Process != nil {
		me.cmd.Process.Kill()
		me.resetStatus()
		me.cmd = nil
	}
}

var findAddressRegex = regexp.MustCompile(`[A-Z][^\/=\&\.\%\- \n]{60,}`)

func FindAddress(anyText string) (string) {
	id := findAddressRegex.FindString(anyText)
	if len(id) > 0 {
		return strings.TrimSpace(id)
	}
	return ""
}

func ReadTableData(str string) [][]string {
	s := "+------"
	start := strings.Index(str, s)
	e := "------+\n"
	end := strings.LastIndex(str, e)
	if start > -1 && end > -1 {
		table := make([][]string, 0)
		str = str[start: end+len(e)]
		str = str[strings.Index(str, "\n"):]
		var line = regexp.MustCompile(`(?m)^\|[^\n]+\|$`)
		var re = regexp.MustCompile(`\|\s+([^\|]+)`)
		for i, match := range line.FindAllString(str, -1) {
			table = append(table, []string{})
			for _, match := range re.FindAllStringSubmatch(match, -1) {
				if len(match) >= 1 {
					table[i] = append(table[i], strings.TrimSpace(match[1]))
				}
			}
		}
		return table
	}
	return nil
}
