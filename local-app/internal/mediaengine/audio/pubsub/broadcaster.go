// Package pubsub broadcasts audio frames and metrics to local subscribers.
package pubsub

import "sync"

type Broadcaster struct {
	mu   sync.Mutex
	subs map[string]map[chan []byte]struct{}
}

func New() *Broadcaster { return &Broadcaster{subs: map[string]map[chan []byte]struct{}{}} }
func (b *Broadcaster) Subscribe(topic string) (<-chan []byte, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := make(chan []byte, 8)
	if b.subs[topic] == nil {
		b.subs[topic] = map[chan []byte]struct{}{}
	}
	b.subs[topic][c] = struct{}{}
	return c, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[topic], c)
		if len(b.subs[topic]) == 0 {
			delete(b.subs, topic)
		}
	}
}

func (b *Broadcaster) Publish(topic string, payload []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.subs[topic] {
		select {
		case c <- payload:
		default:
		}
	}
}
