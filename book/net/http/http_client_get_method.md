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

最后一行 return c.doFollowingRedirects(req, shouldRedirectGet)

进入方法里面去看看具体怎么实现的

``` golang 
func (c *Client) doFollowingRedirects(ireq *Request, shouldRedirect func(int) bool) (resp *Response, err error) {
	var base *url.URL
	redirectChecker := c.CheckRedirect
	if redirectChecker == nil {
		redirectChecker = defaultCheckRedirect
	}
	var via []*Request

	if ireq.URL == nil {
		ireq.closeBody()
		return nil, errors.New("http: nil Request.URL")
	}

	req := ireq
	deadline := c.deadline()

	urlStr := "" // next relative or absolute URL to fetch (after first request)
	redirectFailed := false
	for redirect := 0; ; redirect++ {
		if redirect != 0 {
			nreq := new(Request)
			nreq.Cancel = ireq.Cancel
			nreq.Method = ireq.Method
			if ireq.Method == "POST" || ireq.Method == "PUT" {
				nreq.Method = "GET"
			}
			nreq.Header = make(Header)
			nreq.URL, err = base.Parse(urlStr)
			if err != nil {
				break
			}
			if len(via) > 0 {
				// Add the Referer header.
				lastReq := via[len(via)-1]
				if ref := refererForURL(lastReq.URL, nreq.URL); ref != "" {
					nreq.Header.Set("Referer", ref)
				}

				err = redirectChecker(nreq, via)
				if err != nil {
					redirectFailed = true
					break
				}
			}
			req = nreq
		}

		urlStr = req.URL.String()
		if resp, err = c.send(req, deadline); err != nil {
			if !deadline.IsZero() && !time.Now().Before(deadline) {
				err = &httpError{
					err:     err.Error() + " (Client.Timeout exceeded while awaiting headers)",
					timeout: true,
				}
			}
			break
		}

		if shouldRedirect(resp.StatusCode) {
			// Read the body if small so underlying TCP connection will be re-used.
			// No need to check for errors: if it fails, Transport won't reuse it anyway.
			const maxBodySlurpSize = 2 << 10
			if resp.ContentLength == -1 || resp.ContentLength <= maxBodySlurpSize {
				io.CopyN(ioutil.Discard, resp.Body, maxBodySlurpSize)
			}
			resp.Body.Close()
			if urlStr = resp.Header.Get("Location"); urlStr == "" {
				err = fmt.Errorf("%d response missing Location header", resp.StatusCode)
				break
			}
			base = req.URL
			via = append(via, req)
			continue
		}
		return resp, nil
	}

	method := valueOrDefault(ireq.Method, "GET")
	urlErr := &url.Error{
		Op:  method[:1] + strings.ToLower(method[1:]),
		URL: urlStr,
		Err: err,
	}

	if redirectFailed {
		// Special case for Go 1 compatibility: return both the response
		// and an error if the CheckRedirect function failed.
		// See https://golang.org/issue/3795
		return resp, urlErr
	}

	if resp != nil {
		resp.Body.Close()
	}
	return nil, urlErr
}
```