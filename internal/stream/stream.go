package stream

import (
	"context"
	"net/url"
)

type StreamOption func(s *Stream) error

func NewStream(opts ...StreamOption) (*Stream, error) {
	s := &Stream{}
	for _, opt := range opts {
		err := opt(s)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func WithURL(u url.URL) StreamOption {
	return func(s *Stream) error {
		s.url = u
		return nil
	}
}

type Stream struct {
	url url.URL
}

func (s *Stream) Run(ctx context.Context) error {
	return nil
}
