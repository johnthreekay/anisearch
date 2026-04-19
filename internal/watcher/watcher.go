package watcher

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/john3k/anisearch/internal/qbit"
	"github.com/john3k/anisearch/internal/sonarr"
)

type Watcher struct {
	qbit     *qbit.Client
	sonarr   *sonarr.Client
	interval time.Duration

	mu       sync.Mutex
	tracking map[string]bool
	stop     chan struct{}
}

func New(q *qbit.Client, s *sonarr.Client, interval time.Duration) *Watcher {
	return &Watcher{
		qbit:     q,
		sonarr:   s,
		interval: interval,
		tracking: make(map[string]bool),
		stop:     make(chan struct{}),
	}
}

func (w *Watcher) Start() {
	log.Printf("[watcher] Starting download watcher (polling every %s)", w.interval)
	go w.loop()
}

func (w *Watcher) Stop() {
	close(w.stop)
}

func (w *Watcher) loop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.scan(false)

	for {
		select {
		case <-ticker.C:
			w.scan(true)
		case <-w.stop:
			log.Printf("[watcher] Stopped")
			return
		}
	}
}

func (w *Watcher) scan(triggerOnComplete bool) {
	torrents, err := w.qbit.GetTorrents()
	if err != nil {
		log.Printf("[watcher] Failed to get torrents: %v", err)
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	completedAny := false
	currentHashes := make(map[string]bool)

	for _, t := range torrents {
		hash := strings.ToLower(t.Hash)
		currentHashes[hash] = true

		isDownloading := isActiveState(t.State)
		isComplete := isCompletedState(t.State)

		wasDownloading, tracked := w.tracking[hash]

		if !tracked {
			w.tracking[hash] = isDownloading
			continue
		}

		if wasDownloading && isComplete && triggerOnComplete {
			log.Printf("[watcher] Download completed: %s", t.Name)
			completedAny = true
		}

		w.tracking[hash] = isDownloading
	}

	for hash := range w.tracking {
		if !currentHashes[hash] {
			delete(w.tracking, hash)
		}
	}

	if completedAny {
		w.onComplete()
	}
}

func (w *Watcher) onComplete() {
	time.Sleep(5 * time.Second)

	log.Printf("[watcher] Triggering Sonarr rescan...")
	if err := w.sonarr.RescanAll(); err != nil {
		log.Printf("[watcher] Sonarr rescan failed: %v", err)
	} else {
		log.Printf("[watcher] Sonarr rescan triggered successfully")
	}
}

func isActiveState(state string) bool {
	switch state {
	case "downloading", "stalledDL", "queuedDL", "forcedDL", "metaDL", "allocating", "checkingDL":
		return true
	}
	return false
}

func isCompletedState(state string) bool {
	switch state {
	case "uploading", "stalledUP", "queuedUP", "forcedUP", "pausedUP", "checkingUP":
		return true
	}
	return false
}
