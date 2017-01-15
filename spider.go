package wechat_spider

import (
	"log"
	"net/http"

	"github.com/elazarl/goproxy"
)

type spider struct {
	proxy  *goproxy.ProxyHttpServer
	config *Config
}

var _spider = newSpider()

func Regist(proc Processor) {
	_spider.Regist(proc)
}

func Run(port string) {
	_spider.Run(port)
}

func newSpider() *spider {
	sp := &spider{}
	sp.proxy = goproxy.NewProxyHttpServer()
	sp.proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)
	
	return sp
}

func (s *spider) Regist(proc Processor) {
	s.proxy.OnResponse().DoFunc(ProxyHandle(proc))
}

func (s *spider) Run(port string) {
	log.Println("server will at port:" + port)
	log.Fatal(http.ListenAndServe(":"+port, s.proxy))
}

var (
	rootConfig = &Config{
		Verbose:    false,
		AutoScroll: false,
		Metrics:    false,
	}
)

type Config struct {
	Verbose    bool // Debug
	AutoScroll bool // Auto scroll page to hijack all history articles
	Metrics    bool // Get the metrics:such as Comments and Favors
}

func InitConfig(conf *Config) {
	rootConfig = conf
}
