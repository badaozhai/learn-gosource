package http_day01

import (
	"fmt"
	"net/http"
	"io/ioutil"
)

func Demo1(){
	client := http.DefaultClient
	resp, _ := client.Get("http://sina.com.cn")
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
	//输出:新浪网首页的html源代码
	//<!DOCTYPE html>
	//<!-- [ published at 2016-09-13 21:02:18 ] -->
	//<html>
	//<head>
	//<meta http-equiv="Content-type" content="text/html; charset=utf-8" />
	//<meta http-equiv="X-UA-Compatible" content="IE=edge" />
	//<title>新浪首页</title>
	//<meta name="keywords" content="新浪,新浪网,SINA,sina,sina.com.cn,新浪首页,门户,资讯" />
	//<meta name="description" content="新浪网为全球用户24小时提供全面及时的中文资讯，内容覆盖国内外突发新闻事件、体坛赛事、娱乐时尚、产业资讯、实用信息等，设有新闻、体育、娱乐、财经、科技、房产、汽车等30多个内容频道，同时开设博客、视频、论坛等自由互动交流空间。" />
	//<link rel="mask-icon" sizes="any" href="http://www.sina.com.cn/favicon.svg" color="red">
	//<meta name="stencil" content="PGLS000022" />
	//<meta name="publishid" content="30,131,1" />
	//<meta name="verify-v1" content="6HtwmypggdgP1NLw7NOuQBI2TW8+CfkYCoyeB8IDbn8=" />
	//<meta name="360-site-verification" content="63349a2167ca11f4b9bd9a8d48354541" />
	//<meta name="application-name" content="新浪首页"/>
	//<meta name ="msapplication-TileImage" content="http://i1.sinaimg.cn/dy/deco/2013/0312/logo.png"/>
	//<meta name="msapplication-TileColor" content="#ffbf27"/>
	//<meta name="sogou_site_verification" content="BVIdHxKGrl"/>
	//<link rel="apple-touch-icon" href="http://i3.sinaimg.cn/home/2013/0331/U586P30DT20130331093840.png" />

}
