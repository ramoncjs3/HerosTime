package utils

import (
	"crypto/tls"
	"github.com/imroc/req"
	"net/http"
	"strings"
	"time"
)

var reqHeaders = req.Header{
	"Accept-Encoding": "gzip, deflate",
	"Content-Type":    "application/x-www-form-urlencoded",
	"User-Agent":      "Dalvik/2.1.0 (Linux; U; Android 10.0.1; Galaxy S20+ Build/V417IR)",
}

func ReqPostData(reqUrl string, reqBodys string) (*req.Resp, error) {
	//req.Debug = true
	//reqProxy := req.SetProxyUrl("http://127.0.0.1:8085")
	if !strings.HasPrefix(reqUrl, "http") {
		reqUrl = "http://" + reqUrl
	}

	req.SetTimeout(30 * time.Second)
	req.SetFlags(req.LstdFlags | req.Lcost)
	req.Client().Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	resp, err := req.Post(reqUrl, reqBodys, reqHeaders)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func ReqGetData(reqUrl string) (*req.Resp, error) {
	//req.Debug = true
	//reqProxy := req.SetProxyUrl("http://127.0.0.1:8085")

	req.SetTimeout(30 * time.Second)
	req.SetFlags(req.LstdFlags | req.Lcost)
	req.Client().Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	resp, err := req.Get(reqUrl)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
