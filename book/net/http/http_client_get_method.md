```golang
func Get(url string) (resp *Response, err error) {
	return DefaultClient.Get(url)
}

// Get issues a GET to the specified URL. If the response is one of the
// following redirect codes, Get follows the redirect after calling the
// Client's CheckRedirect function:
//
//    301 (Moved Permanently)
//    302 (Found)
//    303 (See Other)
//    307 (Temporary Redirect)
//
// An error is returned if the Client's CheckRedirect function fails
// or if there was an HTTP protocol error. A non-2xx response doesn't
// cause an error.
//
// When err is nil, resp always contains a non-nil resp.Body.
// Caller should close resp.Body when done reading from it.
//
// To make a request with custom headers, use NewRequest and Client.Do.
func (c *Client) Get(url string) (resp *Response, err error) {
	req, err := NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.doFollowingRedirects(req, shouldRedirectGet)
}
```

看到这里忽然发现 原来的代码可以简化成

```go
	resp, _ := http.Get("http://sina.com.cn")
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
```

* http包里面对外的Get() 函数 里面调用了默认的 DefaultClient
* 并且该函数就直接返回了 return DefaultClient.Get(url)

试着执行一下，肯定没问题的

### 从注释可得到一些信息:

- 此Get()方法 就是 HTTP 中的 GET ,参数URL就是GET的目标地址
- 如果响应状态码有重定向的,就会去调用client的CheckRedirect函数
	* 如果 CheckRedirect 函数执行失败或者HTTP协议出错了，就回返回一个error

- 如果Get()方法返回的是error == nil ，说明http的请求执行成功了,
	* 当error == nil ,resp.Body 不为空
	* 读 resp.Body的时候 读完了记得关闭
- 如果需要自定义一个request,headers ,此路不通,请使用 NewRequest and Client.Do

