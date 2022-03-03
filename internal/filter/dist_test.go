package filter

import (
	"context"
	"testing"

	"github.com/WIZARDISHUNGRY/hls-await/internal/corpus"
)

func TestMinDistFromCorpus(t *testing.T) {
	testPatterns, err := corpus.Load("testpatterns")
	if err != nil {
		t.Fatalf("Load testpatterns: %v", err)
	}
	interesting, err := corpus.Load("interesting")
	if err != nil {
		t.Fatalf("Load interesting: %v", err)
	}

	f := DefaultMinDistFromCorpus(testPatterns)
	ctx := context.Background()
	for name, img := range interesting.ImagesMap() {
		ok, err := f(ctx, img)
		if err != nil {
			t.Fatalf("filter: %v", err)
		}
		if !ok {
			t.Fatalf("filter failed for %s", name)
		}
	}
}

func TestMinDistFromCorpus_rejects_self(t *testing.T) {
	testPatterns, err := corpus.Load("testpatterns")
	if err != nil {
		t.Fatalf("Load testpatterns: %v", err)
	}

	f := DefaultMinDistFromCorpus(testPatterns)
	ctx := context.Background()
	for name, img := range testPatterns.ImagesMap() {
		if name != "FM5muAHXIAMc01i.png" {
			continue
		}
		ok, err := f(ctx, img)
		if err != nil {
			t.Fatalf("filter: %v", err)
		}
		if ok {
			t.Fatalf("filter succeeded for %s", name)
		}
	}
}
