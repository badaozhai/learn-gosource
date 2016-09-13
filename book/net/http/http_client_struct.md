# http.Client{}
先从一个简单的demo讲起比较好:

``` golang
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
```
第一行 : client := http.DefaultClient

net/http/client.go 第81行

// DefaultClient is the default Client and is used by Get, Head, and Post.

var DefaultClient = &Client{}

默认的 Client{}

接下来看看这个 Client{} 是什么？？

顾名思义，http的客户端,现实生活中 http 客户端 最常见的就是我们的浏览器;

所以这玩意可以被用来模拟一个浏览器去发起一个http的请求

``` golang

type Client struct {
	
	Transport RoundTripper
	
	CheckRedirect func(req *Request, via []*Request) error

	Jar CookieJar

	Timeout time.Duration
}
```

#Client 

#### 注释翻译:

Client 就是一个 HTTP Client,他的零值[默认值]是 DefaultClient

DefaultClient 是一个很有用的客户端 他使用DefaultTransport

Client的传输 一般有内部状态(缓存了tcp连接),所以Clients 应该在被重新使用的时候可以复用

而不是去重新创建,Clients 在多个goroutines(go的协程)中使用是安全的

客户端是比 RoundTripper(比如 Transport)要高级的东西,还有 他能处理HTTP 的一些细节

比如 cookies 和 redirects(重定向)
# 属性
- Transport 指定哪一种HTTP 传输机制,默认使用 DefaultTransport
- CheckRedirect 首先这个属性的类型是一个函数
 
	 如果 CheckRedirect 这个属性为 nil 怎么怎么 
	 
	 如果 CheckRedirect 这个属性不为空 那么看他的返回值
	 
	 经过我之前的经验 如果函数返回值返回error 那么就停止在重定向的那里
	 
	 这在需要获取重定向链接的时候很有用
- Jar
 
	 指向了 cookie jar
	
	 如果为nil,cookies 在 http请求和响应的时候都会被忽略
	 
- Timeout
	
	 超时时间 包含 连接时间,读response,redirect（重定向）等所花的所有时间
	 
	 如果设置成0,就是无限长时间
	 



##### 下一步看看 RoundTripper 是什么?

###  RoundTripper 是一个接口,表示一种能够执行简单的 HTTP 协议 并且能后获取请求的响应
#### 注释翻译
 也就是说任何一个东西 只要有这个能力 那他就是 实现了该接口
 
 RoundTripper 必须是协程安全的
 
 RoundTrip 执行一个简单 HTTP事务 ,并返回 响应给 提供的 请求
 
 HTTP事务:我的理解 一应一答
 
 RoundTrip 不用管响应的内容,只要有响应,不管HTTP的状态码是多少
 
  200 也好 404也罢,返回的err 都应该是nil
  
 RoundTrip 除了只能去释放 和 关闭 request,request里的东西一点都不能改
 
 
``` golang
type RoundTripper interface {	
		
	RoundTrip(*Request) (*Response, error)
	
}

```

## links
   * [目录](../../index.md)
   * [client.Get()方法](http_client_get_method.md)


