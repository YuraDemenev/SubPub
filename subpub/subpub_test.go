package subpub

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Проверяем, что NewSubPub создаёт корректный экземпляр SubPub
func TestNewSubPub(t *testing.T) {
	sp := NewSubPub()
	assert.NotNil(t, sp, "NewSubPub should return non-nil SubPub")

	// TODO Убрать?
	// Проверяем, что SubPub реализует интерфейс
	_, ok := sp.(SubPub)
	assert.True(t, ok, "NewSubPub should return a SubPub interface")

	// Проверяем инициализацию полей
	spImpl, ok := sp.(*subPub)
	assert.True(t, ok, "NewSubPub should return *subPub")
	assert.NotNil(t, spImpl.subscribers, "subscribers map should be initialized")
	assert.NotNil(t, spImpl.msgCh, "msgCh should be initialized")
	assert.NotNil(t, spImpl.stopCh, "stopCh should be initialized")
	assert.NotNil(t, spImpl.logger, "logger should be initialized")
	assert.False(t, spImpl.closed, "closed should be false initially")
	assert.Greater(t, cap(spImpl.msgCh), 1, "msgCh should have capacity more than 1")

	// Проверяем, что SubPub можно закрыть
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := sp.Close(ctx)
	assert.NoError(t, err, "Close should succeed")
}

// Проверяем, что воркеры создаются и завершаются
func TestNewSubPubWorkers(t *testing.T) {
	// Создаём SubPub
	sp := NewSubPub()

	// Публикуем тестовое сообщение
	err := sp.Publish("test_subject", "test_message")
	assert.NoError(t, err, "Publish should succeed")

	// Даём воркерам время обработать
	time.Sleep(100 * time.Millisecond)

	// Закрываем SubPub и проверяем завершение воркеров
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err = sp.Close(ctx)
	assert.NoError(t, err, "Close should succeed")
}

// Check Subscribe function
func TestSubscribe(t *testing.T) {
	// Создаём SubPub
	sp := NewSubPub()

	_, err := sp.Subscribe("test", func(msg interface{}) {
		fmt.Println("test")
	})

	//Check error
	assert.NoError(t, err, "Subscribe should succeed")
	spImpl, ok := sp.(*subPub)
	assert.True(t, ok, "NewSubPub should return *subPub")

	//Check count subjects
	assert.Equal(t, 1, len(spImpl.subscribers), "SunPub should has 1 subjects")

	//Add another subscriber with diffrent subject
	_, err = sp.Subscribe("test1", func(msg interface{}) {
		fmt.Println("test1")
	})
	//Check count subjects
	assert.Equal(t, 2, len(spImpl.subscribers), "SunPub should has 2 subjects")

	//add another subscriber with same subject
	_, err = sp.Subscribe("test", func(msg interface{}) {
		fmt.Println("test3")
	})
	//Check count subjects
	assert.Equal(t, 2, len(spImpl.subscribers), "SunPub should has 2 subjects")
	//Check count subscribers
	assert.Equal(t, 2, len(spImpl.subscribers["test"]), "subject test should has 2 subscribers")

	//Check exist
	_, exist := spImpl.subscribers["test1"]
	assert.True(t, exist, "SubPub should has subject test1")

	// Close SubPub
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	assert.NoError(t, sp.Close(ctx), "Close should succeed")
}

func TestSubscribeErrors(t *testing.T) {
	sp := NewSubPub()

	// Empty subject
	_, err := sp.Subscribe("", func(msg interface{}) {})
	assert.Error(t, err, "Subscribe with empty subject should fail")

	// Nil handler
	_, err = sp.Subscribe("test", nil)
	assert.Error(t, err, "Subscribe nil handler should fail")

	// Closed subPub
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	assert.NoError(t, sp.Close(ctx), "Close should succeed")
	_, err = sp.Subscribe("test", func(msg interface{}) {})
	assert.Error(t, err, "Subscribe on closed subPub should fail")
}

func TestUnsubscribe(t *testing.T) {
	//Test 1 Unsubscribe
	sp := NewSubPub()

	sub, err := sp.Subscribe("test", func(msg interface{}) {})
	assert.NoError(t, err, "Subscribe should succeed")

	sub.Unsubscribe()
	spImpl := sp.(*subPub)
	_, exists := spImpl.subscribers["test"]
	assert.False(t, exists, "Subject test should be deleted")

	//Test multiply Unsubscribes
	sub1, err := sp.Subscribe("test", func(msg interface{}) {})
	assert.NoError(t, err, "First Subscribe should succeed")

	sub2, err := sp.Subscribe("test", func(msg interface{}) {})
	assert.NoError(t, err, "First Subscribe should succeed")

	//Unsubscribe first
	sub1.Unsubscribe()

	subs, exist := spImpl.subscribers["test"]
	assert.True(t, exist, "Subject test should still exist")
	assert.Len(t, subs, 1, "Subs should have size 1")

	//Unsubscribe second
	sub2.Unsubscribe()
	subs, exist = spImpl.subscribers["test"]
	assert.False(t, exist, "Subject test should not  exist")
	assert.Len(t, subs, 0, "Subs should have size 0")

	//Unsubscribe second again, check panic (program should return without panic)
	sub2.Unsubscribe()
	subs, exist = spImpl.subscribers["test"]
	assert.False(t, exist, "Subject test should not  exist")
	assert.Len(t, subs, 0, "Subs should have size 0")

	//Check Unsubscribe from not exist subject
	sub, err = sp.Subscribe("test", func(msg interface{}) {})
	assert.NoError(t, err, "Subscribe should succeed")

	// modify subscription, for delete not exist subject
	subImpl := sub.(*subscription)
	subImpl.subject = "not exist subject"

	//Unsubscribe, check panic (program should return without panic)
	subImpl.Unsubscribe()
	//Check test subject exist

	_, exists = spImpl.subscribers["test"]
	assert.True(t, exists, "Original subject test should still exist")

	// close SubPub
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	assert.NoError(t, sp.Close(ctx), "Close should succeed")
}

func TestPublish(t *testing.T) {
	sp := NewSubPub()

	//Publish message to doesn`t exist subject
	err := sp.Publish("test", "test")
	assert.Error(t, err, "Publish shouln`t succeed")

	//For synchronize
	var wg sync.WaitGroup
	received := make([]string, 0, 2)
	var mu sync.Mutex

	// Prepare two subs
	handler := func(msg interface{}) {
		defer wg.Done()
		mu.Lock()
		received = append(received, msg.(string))
		mu.Unlock()
	}

	wg.Add(2)
	sub1, err := sp.Subscribe("test", handler)
	assert.NoError(t, err, "Subscribe should succeed")
	sub2, err := sp.Subscribe("test", handler)
	assert.NoError(t, err, "Subscribe should succeed")

	//Publish message
	err = sp.Publish("test", "test")
	assert.NoError(t, err, "Publish should succeed")

	wg.Wait()

	//Check that both subscribers got message
	assert.Len(t, received, 2, "Both subscribers should get message")
	assert.Contains(t, received, "test", "Received messages should include test")
	sub1.Unsubscribe()
	sub2.Unsubscribe()

	//Check errors when publish
	err = sp.Publish("", "test")
	assert.Error(t, err, "Publish with empty subject should fail")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	assert.NoError(t, sp.Close(ctx), "Close should succeed")
	err = sp.Publish("test_subject", "test_message")
	assert.Error(t, err, "Publish on closed SubPub should fail")
}

// func getLogs(t *testing.T) *logrus.Logger {
// 	// Перехватываем логи
// 	var logBuf bytes.Buffer
// 	logger := logrus.New()
// 	logger.SetOutput(&logBuf)
// 	logger.SetFormatter(&logrus.TextFormatter{
// 		ForceColors:     true,
// 		FullTimestamp:   true,
// 		TimestampFormat: "2006/01/02 15:04:05",
// 	})
// 	logger.SetLevel(logrus.InfoLevel)
// 	return logger
// }
