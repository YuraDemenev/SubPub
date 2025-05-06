package subpub

import (
	"context"
	"errors"
	"log"
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
	msgCh       chan message
	stopCh      chan struct{}
	//Bool flag for check is subPub close
	closed bool
	wg     sync.WaitGroup
	logger *log.Logger
}

type subscription struct {
	//Reference to subPub for accessing subscribers
	sp *subPub
	//Save subject for unsubscribe
	subject string
	handler MessageHandler
	closed  bool
	mu      sync.RWMutex
}

type message struct {
	subject string
	data    interface{}
}

func NewSubPub() SubPub {
	//Initialize subPub
	sp := &subPub{
		subscribers: make(map[string]map[*subscription]struct{}),
		msgCh:       make(chan message, 1000),
		stopCh:      make(chan struct{}),
		logger:      log.New(log.Writer(), "subPub: ", log.LstdFlags),
	}

	//Create workers pool
	countWorkers := 10
	sp.wg.Add(countWorkers)
	for i := 0; i < countWorkers; i++ {
		go func(id int) {
			defer sp.wg.Done()
			for {
				select {
				case msg, ok := <-sp.msgCh:
					if !ok {
						sp.logger.Printf("[INFO] Worker %d: Message channel closed", id)
						return
					}
					//For read map with sunjects
					sp.mu.RLock()

					subs, exist := sp.subscribers[msg.subject]
					sp.mu.RUnlock()

					if !exist {
						sp.logger.Printf("[WARN] Worker %d: subject %s is not exist", id, msg.subject)
						return
					}

					for sub := range subs {
						//For read sub
						sub.mu.RLock()
						if sub.closed {
							sub.mu.RUnlock()
							sp.logger.Printf("[INFO] Worker %d: Subscriber for subject %s is closed", id, msg.subject)
							continue
						}

						handler := sub.handler
						sub.mu.RUnlock()
						//Call handler in other goroutine for  don`t block this
						go handler(msg.data)
					}

				case <-sp.stopCh:
					sp.logger.Printf("[INFO] Worker %d is stopped", id)
					return
				}
			}
		}(i)
	}

	sp.logger.Printf("[INFO] SubPub was initialized with %d workers", countWorkers)
	return sp
}

// Subscribe
func (sp *subPub) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	if subject == "" {
		sp.logger.Printf("[ERROR] Subscribe failed: subject cannot be empty")
		return nil, errors.New("subject cannot be empty")
	}
	if cb == nil {
		sp.logger.Printf("[ERROR] Subscribe failed: message handler cannot be nil")
		return nil, errors.New("message Handler cannot be nil")
	}

	//For work with map without data race
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if sp.closed {
		sp.logger.Printf("[ERROR] Subscribe failed: subpub is closed")
		return nil, errors.New("subPub is closed")
	}

	//Create a subscription
	subscr := &subscription{
		sp:      sp,
		subject: subject,
		handler: cb,
	}

	//Create map for subject if it not exist
	if _, exist := sp.subscribers[subject]; !exist {
		sp.subscribers[subject] = map[*subscription]struct{}{}
	}

	//Add subscription
	sp.subscribers[subject][subscr] = struct{}{}
	sp.logger.Printf("[INFO] Subscribed to subject %s", subject)

	return subscr, nil
}

// Unsubscribe removes the subscription
func (s *subscription) Unsubscribe() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		s.sp.logger.Printf("[INFO] Unsubscribe: Already unsubscribed from subject %s", s.subject)
		return
	}
	s.closed = true

	//Remove subscription from map subscribers
	s.sp.mu.Lock()
	defer s.sp.mu.Unlock()

	subs, exist := s.sp.subscribers[s.subject]
	if !exist {
		s.sp.logger.Printf("[WARN] Unsubscribe: Subject %s does not exist", s.subject)
		return
	}

	//Delete subscriber
	delete(subs, s)
	//delete subject if subject has no subscribers
	if len(subs) == 0 {
		delete(s.sp.subscribers, s.subject)
	}
	s.sp.logger.Printf("[INFO] Unsubscribed from subject %s", s.subject)
}

// Publish
func (sp *subPub) Publish(subject string, msg interface{}) error {
	if subject == "" {
		sp.logger.Printf("[ERROR] Publish failed: subject cannot be empty")
		return errors.New("subject cannot be empty")
	}

	//For work with map without data race
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	if sp.closed {
		sp.logger.Printf("[ERROR] Publish failed: subpub is closed")
		return errors.New("subpub is closed")
	}

	// Send message to centralized queue
	select {
	case sp.msgCh <- message{subject: subject, data: msg}:
		sp.logger.Printf("[INFO] Published message to subject %s", subject)
	case <-sp.stopCh:
		sp.logger.Printf("[ERROR] Publish failed: subpub is closed")
		return errors.New("subpub is closed")
	default:
		sp.logger.Printf("[WARN] Dropped message for subject %s: queue full", subject)
	}

	return nil
}

// Close
func (sp *subPub) Close(ctx context.Context) error {
	sp.logger.Printf("[INFO] Closing SubPub")

	sp.mu.Lock()
	defer sp.mu.Unlock()
	if sp.closed {
		sp.logger.Printf("[INFO] SubPub already closed")
		//TODO поменять на ошибку
		return nil
	}
	sp.closed = true
	close(sp.stopCh)

	// Unsubscribe all subscribers
	for subject, subs := range sp.subscribers {
		for sub := range subs {
			sub.mu.Lock()
			sub.closed = true
			sub.mu.Unlock()
		}
		delete(sp.subscribers, subject)
	}

	// Chanel for send signal that sub pub closed
	done := make(chan struct{})
	//Start new goroutine for react to close channel or end with error
	go func() {
		sp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		sp.logger.Printf("[INFO] SubPub closed successfully")
		return nil
	case <-ctx.Done():
		sp.logger.Printf("[ERROR] SubPub close failed: %v", ctx.Err())
		return ctx.Err()
	}
}

// Задание состоит из 2х частей.
// 1. В первой части требуется реализовать пакет subpub. В этой части задания нужно написать простую шину событий, работающую
// по принципу Publisher-Subscriber.
// Требования к шине:
// • На один subject может подписываться (и отписываться) множество подписчиков.
// Один медленный подписчик не должен тормозить остальных.
// • Нельзя терять порядок порядок сообщений (FIFO очередь).
// • Метод Close должен учитывать переданный контекст. Если он отменен выходим сразу, работающие хендлеры оставляем работать.
// Горутины (если они будут) течь не должны.
