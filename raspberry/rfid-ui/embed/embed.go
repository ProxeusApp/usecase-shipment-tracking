package embed

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	doctmpl "github.com/ProxeusApp/usecase-shipment-tracking/raspberry/rfid-ui"
	"github.com/labstack/echo"
)

func SetupServer() (*doctmpl.MyServer, error) {
	return doctmpl.SetupServer(func(e *echo.Echo) {
		embedded := &Embedded{Asset: Asset}
		doctmpl.ReadAllFile = embedded.Asset2
		e.GET("/static/*", func(c echo.Context) error {
			url := c.Request().URL.String()
			i := strings.LastIndex(url, "?")
			if i != -1 {
				url = url[:i]
			}
			ct := ""
			b, err := embedded.FindAssetWithCT(url, &ct)
			if err == nil {
				c.Blob(http.StatusOK, ct, b)
			}
			return echo.ErrNotFound
		})
	})
}

type Embedded struct {
	Asset func(name string) ([]byte, error)
}

func (emb *Embedded) Asset2(name string) ([]byte, error) {
	if name != "" {
		if strings.HasPrefix(name, "/") {
			return emb.Asset(name[1:])
		} else {
			return emb.Asset(name)
		}
	}
	return nil, os.ErrNotExist
}

func (emb *Embedded) FindAssetWithCT(name string, ct *string) ([]byte, error) {
	if name != "" {
		jj, err := emb.Asset2(name)
		if err != nil {
			return nil, err
		}
		if contentTypeCache == nil {
			contentTypeMutex.Lock()
			if contentTypeCache == nil {
				contentTypeCache = make(map[string]string)
			}
			contentTypeMutex.Unlock()
		}
		contentTypeMutex.RLock()
		cnttype := contentTypeCache[name]
		contentTypeMutex.RUnlock()
		if cnttype == "" {
			li := strings.LastIndex(name, ".")
			if li > 0 {
				ext := name[li:]
				cnttype = ctMap[ext]
			}
			if cnttype == "" {
				cnttype = http.DetectContentType(jj)
			}
			contentTypeMutex.Lock()
			ct2 := contentTypeCache[name]
			if ct2 == "" {
				contentTypeCache[name] = cnttype
			}
			contentTypeMutex.Unlock()
		}
		*ct = cnttype
		return jj, nil
	}
	return nil, echo.ErrNotFound
}

type EmbeddedTemplateLoader struct {
	Embedded *Embedded
}

// Abs calculates the path to a given template. Whenever a path must be resolved
// due to an import from another template, the base equals the parent template's path.
func (htl *EmbeddedTemplateLoader) Abs(base, name string) (absPath string) {
	if base != "" {
		if htl.exists(name) {
			return name
		}
		name = filepath.Join(filepath.Dir(base), name)
	}
	return name
}

func (htl *EmbeddedTemplateLoader) exists(p string) bool {
	buf, err := htl.Embedded.Asset(p)
	if err != nil {
		return false
	}
	return len(buf) > 0
}

// Get returns an io.Reader where the template's content can be read from.
func (htl *EmbeddedTemplateLoader) Get(path string) (io.Reader, error) {
	if path != "" {
		if htl.Embedded != nil && htl.Embedded.Asset != nil {
			buf, err := htl.Embedded.Asset(path)
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(buf), nil
		}
	}
	return nil, echo.ErrNotFound
}

var (
	contentTypeCache map[string]string
	contentTypeMutex sync.RWMutex

	ctMap = map[string]string{
		".js":    "application/javascript",
		".css":   "text/css",
		".gif":   "image/gif",
		".png":   "image/png",
		".tff":   "application/font-sfnt",
		".otf":   "application/font-sfnt",
		".woff":  "application/font-woff",
		".woff2": "application/font-woff",
		".svg":   "image/svg+xml",
		".eot":   "image/png",
	}
)
