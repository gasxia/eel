package router

import (
	"net/http"
	"github.com/kfchen81/eel/log"
	"sync"
	"github.com/kfchen81/eel/handler"
	"strings"
	"fmt"
	"time"
)

type RestResourceRegister struct {
	endpoint2resource map[string]handler.RestResourceInterface
	pool sync.Pool
	sync.RWMutex
}

// Implement http.Handler interface.
func (this *RestResourceRegister) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	log.Infow("request ", "path", req.URL.Path, "method", req.Method)
	
	//get handler.Context from pool
	context := this.pool.Get().(*handler.Context)
	context.Reset(resp, req)
	defer this.pool.Put(context)
	
	endpoint := req.URL.Path
	if endpoint[len(endpoint)-1] != '/' {
		endpoint = endpoint + "/"
	}
	
	resource := this.endpoint2resource[endpoint]
	if resource == nil {
		context.Response.ErrorWithCode(http.StatusNotFound, "resource:not_found", "无效的endpoint", "")
	} else {
		//check resource params
		handler.CheckArgs(resource, context)
		if context.Response.Started {
			context.Response.Flush()
			goto FinishHandle
		}
		
		//pass param check, do prepare
		resource.Prepare(context)
		method := context.Request.Method()
		log.Infow("http request method", "method", method)
		log.Infow("http params", "params", context.Request.Input())
		switch method {
		case "GET":
			resource.Get(context)
		case "PUT":
			resource.Put(context)
		case "POST":
			resource.Post(context)
		case "DELETE":
			resource.Delete(context)
		default:
			http.Error(context.Response.ResponseWriter, "Method Not Allowed", 405)
		}
	}
	
	FinishHandle:
	timeDur := time.Since(startTime)
	statusCode := context.Response.Status
	contentLength := context.Response.ContentLength
	accessLog := fmt.Sprintf("%s - - [%s] \"%s %s %s %d %d\" %f", req.RemoteAddr, startTime.Format("02/Jan/2006 03:04:05"), req.Method, req.RequestURI, req.Proto, statusCode, contentLength, timeDur.Seconds())
	log.Infow(accessLog, "timeDur", timeDur.Seconds(), "status", statusCode)
}

// global resource register
var gRegister *RestResourceRegister = nil

// Create new RestResourceRegister as a http.Handler
func NewRestResourceRegister() *RestResourceRegister {
	if (gRegister == nil) {
		log.Debug("create global RestResourceRegister")
		gRegister = &RestResourceRegister{}
		gRegister.pool.New = func() interface{} {
			return handler.NewContext()
		}
		gRegister.endpoint2resource = make(map[string]handler.RestResourceInterface)
	}
	
	return gRegister
}

func RegisterResource(resource handler.RestResourceInterface) {
	gRegister.Lock()
	defer gRegister.Unlock()
	
	endpoint := resource.Resource()
	pos := strings.LastIndex(endpoint, ".")
	log.Debug(pos)
	endpoint = fmt.Sprintf("/%s/%s/", endpoint[0:pos], endpoint[pos+1:])
	//apiEndpoint := fmt.Sprintf("/%s/%s/", endpoint[0:pos], endpoint[pos+1:])
	gRegister.endpoint2resource[endpoint] = resource
	log.Infow("register rest resource", "endpoint", endpoint)
	log.Debug(gRegister.endpoint2resource)
}

func init() {
	NewRestResourceRegister()
}