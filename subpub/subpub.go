package subpub

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// MessageHandler is a callback function that processes messages delivered to subscribers
type MessageHandler func(msg interface{})

type Subscription interface {
	// Unsubscribe will remove interest in the current subject subscription
	Unsubscribe()
}

type SubPub interface {
	// Subscribe creates an asynchronous queue subscriber on the given subject
	Subscribe(subject string, cb MessageHandler) (Subscription, error)
	// Publish publishes the msg argument to the given subject.
	Publish(subject string, msg interface{}) error
	// Close will shutdown sub-pub system.
	// May be blocked by data delivery until the context is canceled.
	Close(ctx context.Context) error
}

type subPub struct {
	mu sync.RWMutex
	//For save multiply subsciptions for 1 topic and O(1) Delete
	subscribers map[string]map[*subscription]struct{}
	closed      bool
	wg          sync.WaitGroup
}

type subscription struct {
	handler MessageHandler
	queue   chan interface{}
	closed  bool
	mu      sync.RWMutex
}

func NewSubPub() SubPub {
	return &subPub{
		subscribers: make(map[string]map[*subscription]struct{}),
	}
}

// Unsubscribe removes the subscription
func (s *subscription) Unsubscribe() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	close(s.queue)
}

func (sp *subPub) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	if subject == "" {
		return nil, errors.New("subject cannot be empty")
	}
	if cb == nil {
		return nil, errors.New("message Handler cannot be nil")
	}
	if sp.closed {
		return nil, errors.New("subPub is closed")
	}

	//For work with map without data race
	sp.mu.Lock()
	defer sp.mu.Unlock()

	//Create a subscription
	subscr := &subscription{
		handler: cb,
		queue:   make(chan interface{}, 100),
	}

	//Create map for subject if it not exist
	if _, exist := sp.subscribers[subject]; !exist {
		sp.subscribers[subject] = map[*subscription]struct{}{}
	}

	//Add subscription
	sp.subscribers[subject][subscr] = struct{}{}

	//start goroutine to read messages
	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
		for msg := range subscr.queue {
			subscr.mu.RLock()
			if subscr.closed {
				subscr.mu.RUnlock()
				return
			}
			subscr.mu.RUnlock()
			subscr.handler(msg)
		}
	}()

	return subscr, nil
}

// Publish
func (sp *subPub) Publish(subject string, msg interface{}) error {
	if subject == "" {
		return errors.New("subject cannot be empty")
	}
	if sp.closed {
		return errors.New("subpub is closed")
	}

	//For work with map without data race
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	subs, exists := sp.subscribers[subject]
	if !exists {
		return fmt.Errorf("subject :%s is not exist", subject)
	}

	// Publish to all subscribers
	//Select default and write in channel are fast enough, so i don`t use goroutines
	for sub := range subs {
		//Mutext for check is sub closed
		sub.mu.RLock()
		if sub.closed {
			sub.mu.RUnlock()
			continue
		}
		sub.mu.RUnlock()

		select {
		case sub.queue <- msg:
			//TODO
			fmt.Println("Message successfully sent to the queue")
		default:
			//TODO
			fmt.Println("Sub fall in default")
		}
	}

	return nil
}

// Close
func (sp *subPub) Close(ctx context.Context) error {
	sp.mu.Lock()
	if sp.closed {
		sp.mu.Unlock()
		return errors.New("sub pub is closed")
	}
	sp.closed = true

	// Unsubscribe all subscribers
	for subject, subs := range sp.subscribers {
		for sub := range subs {
			sub.Unsubscribe()
		}
		delete(sp.subscribers, subject)
	}
	sp.mu.Unlock()

	// Chanel for send signal that sub pub closed
	done := make(chan struct{})
	//Start new goroutine for react to close channel or end with error
	go func() {
		sp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// isClosed checks if the subscription is closed
// func (s *subscription) isClosed() bool {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
// 	return s.closed
// }

// Задание состоит из 2х частей.
// 1. В первой части требуется реализовать пакет subpub. В этой части задания нужно написать простую шину событий, работающую
// по принципу Publisher-Subscriber.
// Требования к шине:
// • На один subject может подписываться (и отписываться) множество подписчиков.
// Один медленный подписчик не должен тормозить остальных.
// • Нельзя терять порядок порядок сообщений (FIFO очередь).
// • Метод Close должен учитывать переданный контекст. Если он отменен выходим сразу, работающие хендлеры оставляем работать.
// Горутины (если они будут) течь не должны.
