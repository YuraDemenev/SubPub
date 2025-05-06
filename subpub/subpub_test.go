package subpub

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
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

func getLogs(t *testing.T) *logrus.Logger {
	// Перехватываем логи
	var logBuf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuf)
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006/01/02 15:04:05",
	})
	logger.SetLevel(logrus.InfoLevel)
	return logger
}
