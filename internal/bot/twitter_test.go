package bot

import (
	"fmt"
	"testing"
	"time"

	"github.com/dghubble/go-twitter/twitter"
)

func TestParseDate(t *testing.T) {
	tw := &twitter.Tweet{
		CreatedAt: `Wed Feb 23 23:25:53 +0000 2022`,
	}
	tm, err := time.Parse(time.RubyDate, tw.CreatedAt)
	if err != nil {
		t.Fatalf("time.Parse %v", err)
	}
	_ = tm
}
func TestResume(t *testing.T) {
	c := newClient()
	u, tm, err := getLastTweet(c)
	if err != nil {
		if err != nil {
			t.Fatalf("getLastTweet %v", err)
		}
	}
	fmt.Println(u, tm)
}

func TestTZ(t *testing.T) {
	n := time.Now().In(loc)
	fmt.Println(n.Format(time.Kitchen))
	// TODO: https://en.wikipedia.org/wiki/Public_holidays_in_North_Korea
}
