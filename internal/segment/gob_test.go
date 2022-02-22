package segment

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"image"
	"testing"
)

func TestGobEnc(t *testing.T) {
	var network bytes.Buffer        // Stand-in for a network connection
	enc := gob.NewEncoder(&network) // Will write to network.
	dec := gob.NewDecoder(&network) // Will read from network.
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	err := enc.Encode(img)
	if err != nil {
		t.Fatalf("enc %v", err)
	}
	var q image.RGBA
	err = dec.Decode(&q)
	if err != nil {
		t.Fatalf("dec %v", err)
	}
	fmt.Println(q)
}
