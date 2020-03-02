package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/artvel/less"
)

func main() {
	thisPath := "gitlab.blockfactory.com/sytrax/rfid_ui"
	New(&Config{
		GoImportDomain:      "gitlab.blockfactory.com",
		OutputGoPackageName: "embed",
		OutputGoImportPath:  thisPath+"/embed",
		UpdateAllModules:    false,
		Modules: []string{
			thisPath,
		},
		AfterMergeAssetsCallback: func(tmpDir, goImportDomain string) {
		},
		CompileLess:                 false,
		RemoveLessFilesAfterCompile: false,
	})
}

var goSrcPath = ""
var targetSrcPath = ""
var targetImportPath = ""
var targetDomain = ""
var goPath = ""

type Config struct {
	GoImportDomain              string
	OutputGoImportPath          string
	OutputGoPackageName         string // default main
	OutputGoBindataName         string // default bindata.go
	Modules                     []string
	UpdateAllModules            bool
	CompileLess                 bool
	RemoveLessFilesAfterCompile bool
	AfterMergeAssetsCallback    func(tmpDir, targetDomain string)
}

func New(conf *Config) {
	goPath = os.Getenv("GOPATH")
	if goPath == "" || strings.TrimSpace(goPath) == "" {
		panic(errors.New("GOPATH is not set! Example: $> export GOPATH=~/go/"))
	}
	if conf == nil {
		panic(errors.New("conf can't be nil"))
	}
	targetDomain = conf.GoImportDomain
	targetImportPath = conf.OutputGoImportPath
	gitRedirectHTTPToSSH(targetDomain)
	goSrcPath = goPath + string(os.PathSeparator) + "src" + string(os.PathSeparator)
	ensureBindataExists()
	tmpDir, err := ioutil.TempDir("", "assets")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)
	fmt.Println(tmpDir)
	if conf.UpdateAllModules {
		if len(conf.Modules) > 0 {
			updateAll(conf.Modules)
		}
	}
	if len(conf.Modules) > 0 {
		for _, mod := range conf.Modules {
			mergeToFactoryDir("dist/view", "view", tmpDir, mod)
			mergeToFactoryDir("dist/static", "static", tmpDir, mod)
		}
	}

	if conf.AfterMergeAssetsCallback != nil {
		conf.AfterMergeAssetsCallback(tmpDir, targetDomain)
	}
	CompileLessFiles(conf, tmpDir)
	targetSrcPath = GetGoPath(targetImportPath)
	if !strings.HasSuffix(tmpDir, string(os.PathSeparator)) {
		tmpDir += string(os.PathSeparator)
	}
	pkgName := conf.OutputGoPackageName
	if pkgName == "" {
		pkgName = "main"
	}
	targetSrcPath, err := filepath.Abs(targetSrcPath)
	if err != nil {
		panic(err)
	}

	if conf.OutputGoBindataName == "" {
		conf.OutputGoBindataName = "bindata.go"
	}
	gobindataCmd := exec.Command("go-bindata", "-pkg", pkgName, "-o", filepath.Join(targetSrcPath, conf.OutputGoBindataName), "-prefix", tmpDir, tmpDir+"...")
	gobindataCmd.Dir = targetSrcPath
	a, err := gobindataCmd.Output()
	fmt.Println(string(a))
	if err != nil {
		fmt.Println(string(a))
		panic(err)
	}
	fmt.Println("targetImportPath", targetSrcPath)
}

func CompileLessFiles(conf *Config, tmpDir string) {
	//lookup for css links in html files
	if conf.CompileLess {
		//fmt.Println(tmpDir)
		cssLinks, err := lookupLinkedCssFiles(tmpDir)
		//fmt.Println(cssLinks)
		if len(cssLinks) > 0 {
			for link := range cssLinks {
				cssLink := filepath.Join(tmpDir, link)
				//fmt.Println(cssLink)
				lessPath := strings.Replace(cssLink, ".css", ".less", 1)
				if _, err = os.Stat(lessPath); err == nil {
					//fmt.Println("resolve less", lessPath)
					bb := resolveLess(lessPath, []string{tmpDir, filepath.Join(tmpDir, "static"), filepath.Join(tmpDir, "static", "css")}...)
					if len(bb) > 0 {
						err = ioutil.WriteFile(cssLink, bb, 0775)
					}
				}
			}
		}
		if conf.RemoveLessFilesAfterCompile {
			fmt.Println(removeAllLessFilesIn(filepath.Join(tmpDir, "static")))
		}
	}
}

func resolveLess(path string, moreStaticDirs ...string) []byte {
	w := new(bytes.Buffer)
	err := less.Compile(path, w, moreStaticDirs...)
	if err != nil {
		//fmt.Println(err)
		return []byte("")
	}
	return w.Bytes()
}

func lookupLinkedCssFiles(dir string) (map[string]bool, error) {
	//var re = regexp.MustCompile(`href="(.*\.css)`)
	//links := make(map[string]bool)
	//grep -rhP "" ./view
	//ndir, err := filepath.Abs(dir)
	//if err != nil {
	//	return nil, err
	//}
	return lookupLinkedCssFilesNative(dir)
	//fmt.Println(`'<link(.*?)? href\=\"(.*\.css)(.*)?\"'`)
	////cmd := exec.Command(`grep -rhP '<link(.*?)? href\=\"(.*\.css)(.*)?\"'`, ndir)
	//cmd := exec.Command("grep", "-rhP", `'<link(.*?)? href="(.*.css)(.*)?"'`, ndir)
	//cmd.Dir = targetSrcPath
	//a,err := cmd.Output()
	//if err != nil {
	//fmt.Println(a, err)
	//return nil, err}
	//for _, m := range strings.Split(string(a), "\n") {
	//	if !strings.Contains(m, "integrity") {
	//		match := re.FindAllStringSubmatch(m, -1)
	//		if len(match) > 0 && len(match[0]) > 1 {
	//			links[match[0][1]] = true
	//		}
	//	}
	//}
	//return links, nil
}

func lookupLinkedCssFilesNative(searchDir string) (map[string]bool, error) {
	var re1 = regexp.MustCompile(`href="(.*\.css)`)
	var re = regexp.MustCompile(`<link(.*?)? href\=\"(.*\.css)(.*)?\"`)
	//fileList := []string{}
	links := make(map[string]bool)
	filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		bts, err := ioutil.ReadFile(path)
		if err == nil {
			if strings.HasSuffix(path, ".html") {
				for _, b := range re.FindAll(bts, -1) {
					ab := string(b)
					if !strings.Contains(ab, "integrity") {
						match := re1.FindAllStringSubmatch(ab, -1)
						if len(match) > 0 && len(match[0]) > 1 {
							links[match[0][1]] = true
						}
					}
				}
			}
		}
		//fileList = append(fileList, path)
		return nil
	})

	//for _, file := range fileList {
	//	fmt.Println(file)
	//}
	return links, nil
}

func removeAllLessFilesIn(dir string) error {
	//find /home/ave/go/src/webui/static -name "*.less"
	ndir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	cmd := exec.Command("find", ndir, "-name", "*.less")
	cmd.Dir = targetSrcPath
	a, err := cmd.Output()
	if err != nil {
		return err
	}
	for _, m := range strings.Split(string(a), "\n") {
		p, err := filepath.Abs(m)
		if err != nil {
			return err
		}
		err = os.Remove(p)
	}
	return err
}

func gitRedirectHTTPToSSH(domain string) {
	cmd := exec.Command("git", "config", "--global", fmt.Sprintf("url.git@%s:.insteadOf", domain), fmt.Sprintf("https://%s/", domain))
	err := cmd.Run()
	if err != nil {
		fmt.Println("git not installed???")
		panic(err)
	}
}

func mergeToFactoryDir(dir, destDirName, tmpDir, importPath string) {
	destDir := filepath.Join(tmpDir, destDirName)
	srcDir := filepath.Join(GetGoPath(importPath), dir)
	if dirExists(srcDir) {
		fmt.Println("exists", srcDir)
		err := CopyDir(srcDir, destDir)
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Println("not exists", srcDir)
	}
}

func dirExists(p string) bool {
	_, err := os.Stat(p)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	fmt.Println(err)
	return false
}

func GetGoPath(importPath string) string {
	return goSrcPath + rplpath(importPath)
}

func ensureBindataExists() {
	iups := []string{
		"github.com/go-bindata/go-bindata",
		"github.com/elazarl/go-bindata-assetfs",
	}
	updateAll(iups)
	setGoBinToPATH()
	if exeNotFound("go-bindata") {
		goInstall("github.com/go-bindata/go-bindata/go-bindata")
	}

	if exeNotFound("go-bindata-assetfs") {
		goInstall("github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs")
	}
}

func setGoBinToPATH() {
	gbin := goPath + string(os.PathSeparator) + "bin"
	if !strings.Contains(os.Getenv("PATH"), gbin) {
		os.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+gbin)
	}
}

func goInstall(c string) error {
	g := goSrcPath + rplpath(c)
	cc := exec.Command("go", "install")
	cc.Dir = g
	_, err := cc.Output()
	return err
}

func rplpath(c string) string {
	return strings.Replace(c, "/", string(os.PathSeparator), -1)
}

func exeNotFound(cmd string) bool {
	_, err := exec.Command(cmd).Output()
	return err != nil && strings.Contains(err.Error(), exec.ErrNotFound.Error())
}

func updateAll(ar []string) {
	for _, item := range ar {
		_, err := goGetUpdate(item)
		if err != nil {
			fmt.Println("error when trying to 'go get -u", item, err)
		}

	}
}

func goGetUpdate(importPath string) ([]byte, error) {
	if strings.HasSuffix(importPath, "/") {
		importPath += "..."
	} else {
		importPath += "/..."
	}
	return exec.Command("go", "get", "-u", importPath).Output()
}

// CopyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file. The file mode will be copied from the source and
// the copied data is synced/flushed to stable storage.
func CopyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return
	}

	err = out.Sync()
	if err != nil {
		return
	}

	si, err := os.Stat(src)
	if err != nil {
		return
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return
	}

	return
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped.
func CopyDir(src string, dst string) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if err != nil {
		err = os.MkdirAll(dst, si.Mode())
		if err != nil {
			return
		}
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}

			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return
			}
		}
	}

	return
}
