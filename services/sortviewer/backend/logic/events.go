package logic

import "sync"

// Event tells watchers of a collection that something changed: "piece" (with the
// piece), "run", "plan", "unknowns" or "photos". Pages refetch what they show.
type Event struct {
	Type         string `json:"type"`
	CollectionID int64  `json:"collection_id"`
	Piece        *Piece `json:"piece,omitempty"`
}

// Hub fans events out to the browsers watching each collection.
type Hub struct {
	mu   sync.Mutex
	subs map[int64]map[chan Event]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[int64]map[chan Event]struct{}{}} }

// Subscribe watches a collection; call the returned func to stop.
func (h *Hub) Subscribe(collectionID int64) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	h.mu.Lock()
	if h.subs[collectionID] == nil {
		h.subs[collectionID] = map[chan Event]struct{}{}
	}
	h.subs[collectionID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs[collectionID], ch)
		if len(h.subs[collectionID]) == 0 {
			delete(h.subs, collectionID)
		}
		h.mu.Unlock()
	}
}

// Publish sends an event to a collection's watchers. A watcher too slow to keep up
// misses it rather than holding up the sorter's push.
func (h *Hub) Publish(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[e.CollectionID] {
		select {
		case ch <- e:
		default:
		}
	}
}
