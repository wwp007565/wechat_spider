package main

import (
	// "math/rand"
	"fmt"
	spider "github.com/sundy-li/wechat_spider"
)

func main() {
	var port = "8899"
	spider.InitConfig(&spider.Config{
		Verbose: false,
		AutoScroll: true,
	})
	spider.Regist(&CustomProcessor{})
	spider.Run(port)

}

//Just to implement Output Method of interface{} Processor
type CustomProcessor struct {
	spider.BaseProcessor
}

func (c *CustomProcessor) Output() {
	// Just print the length of result urls
	println("result urls size =>", len(c.Result()))
	for i, r := range c.Result() {
		fmt.Println(i, r.Url)
		// fmt.Println(r.CoverImage)
	}
	// You can dump the get the html from urls and save to your database
}

// NextBiz hijack the script, set the location to next url after 2 seconds
// func (c *CustomProcessor) NextBiz(currentBiz string) string {
// 	// Random select
// 	return _bizs[rand.Intn(len(_bizs))]
// }

// var (
// 	_bizs = []string{"MzAwODI2OTA1MA==", "MzA5NDk4ODI4Mw==", "MjM5MjEyOTEyMQ=="}
// )
