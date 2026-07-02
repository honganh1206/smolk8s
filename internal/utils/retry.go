package utils

import (
	"fmt"
	"net/http"
)

func RetryHTTP(f func(string) (*http.Response, error), url string) (*http.Response, error) {
	count := 10
	var resp *http.Response
	var err error
	for i := 0; i < count; i++ {
		resp, err = f(url)
		if err != nil {
			fmt.Printf("Error calling URL %v\n", url)
		} else {
			break
		}
	}
	return resp, err
}
