package helper

import (
	"regexp"
	"strings"
	"io/ioutil"
	"os"
	"fmt"
	"io"
)

var reIni = regexp.MustCompile(`(?m)(\w+) = ([^\n]+)`)
var iniFile = "/home/pi/MFRC522-python/sytrax.ini"
//var iniFile = "/home/ave/go/src/gitlab.blockfactory.com/sytrax/rfid_ui/helper/sytrax.ini"
func ReadIni() (map[string]string){
	return readIni(read())
}

func read() string {
	f, err := os.Open(iniFile)
	if err != nil {
		return ""
	}
	var b []byte
	b, err = ioutil.ReadAll(f)
	f.Close()
	if err != nil {
		return ""
	}
	return string(b)
}

func WriteIni(d map[string]string) (error){
	c := read()
	if len(c) == 0 {
		c = `[MainProd]`
		c += "\n"
		for k, v := range d {
			c += fmt.Sprintf("%s = %s\n", k, v)
		}
	} else {
		for k, v := range d {
			//c += k+" = "+v+"\n"
			r := regexp.MustCompile(`(?m)(`+k+`) = ([^\n]+)`)
			m := r.FindAllString(c, -1)
			if len(m) > 0 {
				c = strings.Replace(c, m[0], fmt.Sprintf("%s = %s", k, v), 1)
			}else{
				c += fmt.Sprintf("%s = %s\n", k, v)
			}
		}
	}
	f, err := os.OpenFile(iniFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	n, err := f.WriteString(c)
	if err == nil && n < len(c) {
		err = io.ErrShortWrite
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return nil
}

func readIni(str string) (map[string]string){
	res := make(map[string]string)
	for _, match := range reIni.FindAllStringSubmatch(str, -1) {
		if len(match)>=3 {
			res[strings.TrimSpace(match[1])] = strings.TrimSpace(match[2])
		}
	}
	return res
}

