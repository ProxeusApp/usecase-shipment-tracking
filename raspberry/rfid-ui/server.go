package doctmpl

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ProxeusApp/usecase-shipment-tracking/raspberry/rfid-ui/helper"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	ReadAllFile = func(path string) ([]byte, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return ioutil.ReadAll(f)
	}
)

type MyServer struct {
	quit chan os.Signal
}

func (ms *MyServer) Close() {
	if ms.quit != nil {
		ms.quit <- os.Interrupt
	}
}

func StartServer(e *echo.Echo, addr string, afterStart func(), onShutdown func()) (*MyServer, error) {
	e.HideBanner = true
	var err error
	ms := &MyServer{quit: make(chan os.Signal)}
	// Start server
	go func() {
		fmt.Println("starting at", addr)
		if err = e.Start(addr); /*e.StartAutoTLS(":443")*/ err != nil {
			fmt.Println("shutting down the server cause of: ", err)
			ms.quit <- os.Interrupt
		}
	}()
	if afterStart != nil {
		go afterStart()
	}
	signal.Notify(ms.quit, os.Interrupt)
	<-ms.quit
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	shutdownWebListener(e, &ctx)
	if onShutdown != nil {
		onShutdown()
	}
	return ms, err
}

func shutdownWebListener(e *echo.Echo, ctx *context.Context) {
	if err := e.Server.Shutdown(*ctx); err != nil {
		e.Logger.Fatal(err)
	}
	if err := e.TLSServer.Shutdown(*ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

var mySytraxPythonHelper *helper.SytraxPhytonHandler

func SetupServer(beforeStart func(ec *echo.Echo)) (*MyServer, error) {
	port := flag.String("p", "58082", "Port")
	host := flag.String("h", "127.0.0.1", "Host")
	flag.Parse()
	var err error

	if err != nil {
		panic(err)
	}
	e := echo.New()
	gob.Register(map[string]interface{}{})
	sessionStore := sessions.NewCookieStore([]byte("secret_Dummy_1234"), []byte("12345678901234567890123456789012"))
	e.Use(session.Middleware(sessionStore))

	e.Pre(middleware.Secure())

	mySytraxPythonHelper = helper.New()
	e.GET("/", func(c echo.Context) error {
		mySytraxPythonHelper.Kill()
		b, err := ReadAllFile("view/dev.html")
		if err != nil {
			return c.NoContent(http.StatusNotFound)
		}
		return c.HTMLBlob(http.StatusOK, b)
	})
	e.GET("/cmd/:nr", func(c echo.Context) error {
		nr := c.Param("nr")
		if nr == "-1" {
			mySytraxPythonHelper.Kill()
			return c.NoContent(http.StatusOK)
		}
		consingmentID := c.QueryParam("cid")
		if (nr == "3" || nr == "2") && len(consingmentID) == 0 {
			return c.NoContent(http.StatusBadRequest)
		}
		go func() {
			mySytraxPythonHelper.Run(nr, consingmentID)
		}()
		return c.NoContent(http.StatusOK)
	})
	e.GET("/cmd/pull/:nr", func(c echo.Context) error {
		nr := c.Param("nr")
		return c.JSON(http.StatusOK, mySytraxPythonHelper.Get(nr))
	})
	e.GET("/cmd/status/:nr", func(c echo.Context) error {
		nr := c.Param("nr")
		return c.JSON(http.StatusOK, mySytraxPythonHelper.Status(nr))
	})
	e.GET("/ini", func(c echo.Context) error {
		return c.JSON(http.StatusOK, helper.ReadIni())
	})
	e.POST("/ini", func(c echo.Context) error {
		b, err := ioutil.ReadAll(c.Request().Body)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		c.Request().Body.Close()
		d := make(map[string]string)
		err = json.Unmarshal(b, &d)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		err = helper.WriteIni(d)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.NoContent(http.StatusOK)
	})

	if beforeStart == nil {
		panic(errors.New("beforeStart can't be nil!"))
	}
	beforeStart(e)
	//web.StaticSetup(system, e, "/static", "static", moreAssetDirs...)

	return StartServer(e, (*host)+":"+(*port), nil, func() {
		fmt.Println("shutting down the server")
		mySytraxPythonHelper.Kill()
	})
}
