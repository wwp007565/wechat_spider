package wechat_spider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/palantir/stacktrace"
)

type Processor interface {
	// Core method
	Process(resp *http.Response, ctx *goproxy.ProxyCtx) ([]byte, error)
	// NextBiz
	NextBiz(currentBiz string) string
	// Result urls
	Result() []*WechatResult
	// Output
	Output()
	// Sleep method to avoid the req control of wechat
	Sleep()
}

type BaseProcessor struct {
	req    *http.Request
	lastId string
	data   []byte
	result []*WechatResult
}

type (
	WechatResult struct {
		Mid string
		// url
		Url string

		_URL *url.URL
		CoverImage string
		// Three below is TODO
		Data       string
		Appmsgstat *MsgStat `json:"appmsgstat"`
		Comments   []*Comment
	}
	MsgStat struct {
		ReadNum     int `json:"read_num"`
		LikeNum     int `json:"like_num"`
		RealReadNum int `json:"real_read_num"`
	}
	Comment struct {
	}
)

var (
	replacer = strings.NewReplacer(
		"\t", "", " ", "",
		"&quot;", `"`, "&nbsp;", "",
		`\\`, "", "&amp;amp;", "&",
		"&amp;", "&", `\`, "",
	)

	urlRegex    = regexp.MustCompile("http://mp.weixin.qq.com/s?[^#]*")
	idRegex     = regexp.MustCompile(`"id":(\d+)`)
	coverImageRegex = regexp.MustCompile("http://mmbiz.qpic.cn/[^,]*")
	// jsonMsgRegex = regexp.MustCompile(`{"list":[^';]*`)
	jsonMsgRegex = regexp.MustCompile(`{"list":[^';]*`)
	MsgNotFound = errors.New("MsgLists not found")
)

func NewBaseProcessor() *BaseProcessor {
	return &BaseProcessor{}
}

func (p *BaseProcessor) init(req *http.Request, data []byte) (err error) {
	p.req = req
	p.data = data
	fmt.Println("Running a new wechat processor, please wait...")
	return nil
}
func (p *BaseProcessor) Process(resp *http.Response, ctx *goproxy.ProxyCtx) (data []byte, err error) {

	var buf bytes.Buffer
	if _, err = buf.ReadFrom(resp.Body); err != nil {
		return
	}
	if err = resp.Body.Close(); err != nil {
		return
	}

	data = buf.Bytes()
	if err = p.init(ctx.Req, data); err != nil {
		return
	}

	if err = p.processMain(); err != nil {
		return
	}

	if rootConfig.AutoScroll {
		if err = p.processPages(); err != nil {
			return
		}
	}

	//gen id
	for _, r := range p.result {
		r._URL, _ = url.Parse(r.Url)
	}

	// TODO
	if rootConfig.Metrics {
		for _, r := range p.Result() {
			err = p.processStat(r)
			if err != nil {
				println(err.Error())
			}
		}
	}
	return
}

func (p *BaseProcessor) NextBiz(currentBiz string) string {
	return ""
}

func (p *BaseProcessor) Sleep() {
	time.Sleep(50 * time.Millisecond)
}

func (p *BaseProcessor) Result() []*WechatResult {
	return p.result
}

func (p *BaseProcessor) Output() {
	urls := []string{}
	fmt.Println("result => [")
	for _, r := range p.result {
		urls = append(urls, r.Url)
	}
	fmt.Println(strings.Join(urls, ","))
	fmt.Println("]")
}

//Parse the html
func (p *BaseProcessor) processMain() error {
	p.result = make([]*WechatResult, 0, 100)
	buffer := bytes.NewBuffer(p.data)
	var msgs string
	str, err := buffer.ReadString('\n')
	for err == nil {
		if strings.Contains(str, "msgList = ") {
			msgs = str
			break
		}
		str, err = buffer.ReadString('\n')
	}
	if msgs == "" {
		return stacktrace.Propagate(MsgNotFound, "Failed parse main")
	}
	msgs = replacer.Replace(msgs)
	// fmt.Println(msgs)
	jsonMsgs := jsonMsgRegex.FindAllString(msgs, -1)
	fmt.Println(jsonMsgs)
	urls := urlRegex.FindAllString(msgs, -1)
	if len(urls) < 1 {
		return stacktrace.Propagate(MsgNotFound, "Failed find url in  main")
	}
	coverImgs := coverImageRegex.FindAllString(msgs, -1)  //所有文章配图
	if len(urls) != len(coverImgs) {
		fmt.Println(len(urls), len(coverImgs))
		return stacktrace.Propagate(MsgNotFound, "Failed urls num not equal coverImgs num")
	}
	p.result = make([]*WechatResult, len(urls))
	var cover string
	for i, u := range urls {
		cover = strings.Replace(coverImgs[i], "\"", "", 1)
		p.result[i] = &WechatResult{Url: u, CoverImage: cover}
	}

	idMatcher := idRegex.FindAllStringSubmatch(msgs, -1)
	if len(idMatcher) < 1 {
		return stacktrace.Propagate(MsgNotFound, "Failed find id in  main")
	}
	p.lastId = idMatcher[len(idMatcher)-1][1]
	return nil
}

func (p *BaseProcessor) processPages() (err error) {
	var pageUrl = p.genPageUrl()
	p.logf("process pages....")
	req, err := http.NewRequest("GET", pageUrl, nil)
	if err != nil {
		return stacktrace.Propagate(err, "Failed new page request")
	}
	for k, _ := range p.req.Header {
		req.Header.Set(k, p.req.Header.Get(k))
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return stacktrace.Propagate(err, "Failed get page response")
	}
	bs, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	str := replacer.Replace(string(bs))
	result := urlRegex.FindAllString(str, -1)
	if len(result) < 1 {
		return stacktrace.Propagate(err, "Failed get page url")
	}
	coverImgs := coverImageRegex.FindAllString(str, -1)  //获取所有文章配图
	if len(result) != len(coverImgs) {
		return stacktrace.Propagate(MsgNotFound, "Failed urls num not equal coverImgs num")
	}
	idMatcher := idRegex.FindAllStringSubmatch(str, -1)
	if len(idMatcher) < 1 {
		return stacktrace.Propagate(err, "Failed get page id")
	}
	p.lastId = idMatcher[len(idMatcher)-1][1]
	p.logf("Page Get => %d,lastid: %s", len(result), p.lastId)
	var cover string
	for i, u := range result {
		cover = strings.Replace(coverImgs[i], "\"", "", 1)
		p.result = append(p.result, &WechatResult{Url: u, CoverImage: cover})
	}
	if p.lastId != "" {
		p.Sleep()
		return p.processPages()
	}
	return nil
}

func (p *BaseProcessor) processStat(r *WechatResult) (err error) {
	mid := r._URL.Query().Get("mid")
	statUrl := p.genStatUrl(mid)
	println("==>", statUrl)
	req, err := http.NewRequest("POST", statUrl, nil)
	if err != nil {
		return stacktrace.Propagate(err, "Failed new stat request")
	}
	for k, _ := range p.req.Header {
		req.Header.Set(k, p.req.Header.Get(k))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return stacktrace.Propagate(err, "Failed get stat response")
	}
	bs, _ := ioutil.ReadAll(resp.Body)
	println(string(bs))
	defer resp.Body.Close()
	err = json.Unmarshal(bs, r)
	if err != nil {
		return stacktrace.Propagate(err, "Failed get unmarshel stats")
	}
	return nil
}

func (p *BaseProcessor) genPageUrl() string {
	urlStr := "http://mp.weixin.qq.com/mp/getmasssendmsg?" + p.req.URL.RawQuery
	urlStr += "&frommsgid=" + p.lastId + "&f=json&count=100"
	return urlStr
}

func (p *BaseProcessor) genStatUrl(mid string) string {
	urlStr := "http://mp.weixin.qq.com/mp/getappmsgext?" + p.req.URL.RawQuery

	values := url.Values{}
	values.Add("mid", mid)
	values.Add("comment_id", "111")
	values.Add("devicetype", "android-22")
	values.Add("version", "/mmbizwap/zh_CN/htmledition/js/appmsg/index32e586.js")
	values.Add("f", "json")

	urlStr += "&" + values.Encode()
	return urlStr
}

func (P *BaseProcessor) logf(format string, msg ...interface{}) {
	if rootConfig.Verbose {
		Logger.Printf(format, msg...)
	}
}
