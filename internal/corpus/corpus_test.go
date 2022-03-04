package corpus

import (
	"testing"
)

func TestLoadFS(t *testing.T) {
	c, err := LoadFS("artifacts")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	imgs := c.Images()
	if len(imgs) == 0 {
		t.Fatal("len(imgs)==0")
	}
}
func TestLoadEmbedded(t *testing.T) {
	c, err := LoadEmbedded("testpatterns")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	imgs := c.Images()
	if len(imgs) == 0 {
		t.Fatal("len(imgs)==0")
	}
}
