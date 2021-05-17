package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestProxy(t *testing.T) {
	ctx := context.Background()
	target, _ := url.Parse("https://www.yahoo.com/")
	u, err := NewSingleHostReverseProxy(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(u)
	r, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(r)
}
