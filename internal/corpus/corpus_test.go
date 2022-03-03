package corpus

import (
	"testing"
)

func TestLoad(t *testing.T) {
	c, err := Load("artifacts")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	imgs := c.Images()
	if len(imgs) == 0 {
		t.Fatal("len(imgs)==0")
	}
}
