package main

import (
	"context"
	"log"
	"sync"
	"time"
)

type LastWriteMap struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.Mutex
	m  map[string]time.Time

	drained bool
	timer   *time.Timer
}

func NewLastWriteMap(ctx context.Context, do func(key string, deadline time.Time) error) *LastWriteMap {
	ctx, cancel := context.WithCancel(ctx)
	lwm := &LastWriteMap{
		ctx:     ctx,
		cancel:  cancel,
		m:       make(map[string]time.Time),
		timer:   time.NewTimer(0),
		drained: false,
	}

	<-lwm.timer.C
	lwm.drained = true

	lwm.wg.Add(1)
	go func() {
		defer lwm.wg.Done()
		lwm.routine(do)
	}()
	return lwm
}

func (lwm *LastWriteMap) Close() {
	lwm.cancel()
	closeTimer(lwm.timer, &lwm.drained)
	lwm.wg.Wait()
}

func (lwm *LastWriteMap) peekNextDeadline() (key string, deadline time.Time, ok bool) {
	if len(lwm.m) == 0 {
		return "", time.Time{}, false
	}

	var (
		minDeadline time.Time
		minKey      string
	)

	for key, deadline := range lwm.m {
		if deadline.Before(minDeadline) || minDeadline.IsZero() {
			minDeadline = deadline
			minKey = key
		}
	}
	return minKey, minDeadline, true
}

func (lwm *LastWriteMap) popNextDeadline() (key string, deadline time.Time, ok bool) {
	key, deadline, ok = lwm.peekNextDeadline()
	if ok {
		delete(lwm.m, key)
	}
	return key, deadline, ok
}

func (lwm *LastWriteMap) Set(key string, deadline time.Time) {
	lwm.mu.Lock()
	defer lwm.mu.Unlock()

	lwm.m[key] = deadline
	lwm.resetTimer()
}

func (lwm *LastWriteMap) routine(do func(key string, deadline time.Time) error) {
	for {
		select {
		case <-lwm.ctx.Done():
			log.Println("context done, stopping routine")
			return
		case <-lwm.timer.C:
			func() {
				lwm.mu.Lock()

				lwm.drained = true
				k, v, ok := lwm.popNextDeadline()
				if !ok {
					log.Println("no dealines, sleeping...")
					lwm.mu.Unlock()
					return
				}
				lwm.mu.Unlock()

				// do not lock here, cuz we may be manipulating the map
				// from within that function
				log.Println("processing:", k)
				err := do(k, v)
				if err != nil {
					log.Println("error while processing:", err)
				}

				lwm.mu.Lock()
				defer lwm.mu.Unlock()
				d := lwm.resetTimer()
				if d > 0 {
					log.Println("next deadline in:", d)
				} else {
					log.Println("no more deadlines left, sleeping...")
				}
			}()
		}
	}
}

func (lwm *LastWriteMap) resetTimer() time.Duration {
	_, v, ok := lwm.peekNextDeadline()
	if !ok {
		return 0
	}
	d := time.Until(v)
	resetTimer(lwm.timer, d, &lwm.drained)
	return d
}
