package rabbitmq

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ConnectionManager struct {
	connection 	*amqp.Connection
	done		chan bool
	logger		*slog.Logger
	mutex		sync.RWMutex
	once  		sync.Once
	ready		chan bool
	url			string
}

func NewConnectionManager(url string, logger *slog.Logger) *ConnectionManager {
	m := &ConnectionManager{
		done:		make(chan bool),
		logger: 	logger.With(slog.String("component", "RabbitMQManager")),
		ready:		make(chan bool, 1),
		url:		url,
	}

	go m.handleReconnect()
	return m
}

func (m *ConnectionManager) handleReconnect() {
	m.logger.Info("Connection manager started")

	const maxBackoff = 5 * time.Minute
	backoff := 1 * time.Second

	for {
		select {
		case <-m.done:
			m.logger.Info("Connection manager stopped")
			return
		default:
			if !m.isConnected() {
				m.logger.Info("Attempting to connect to RabbitMQ...")
				if err := m.connect(); err != nil {
					m.logger.Error("Failed to connect, retrying...", "error", err, "next_try_in", backoff)
					time.Sleep(backoff)
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					continue
				}
				m.logger.Info("Connection established!")
				backoff = 1 * time.Second
				select {
				case m.ready <- true:
				default:
				}
			}
		}

		if m.isConnected() {
			notifyClose := make(chan *amqp.Error, 1)
			m.connection.NotifyClose(notifyClose)

			select {
			case <-m.done:
				m.logger.Info("Connection manager stopping while connected")
				return
			case err := <-notifyClose:
				m.logger.Warn("Connection lost. Reconnecting...", "error", err)
				m.setConnection(nil)
			}
		}
	}
}

func (m *ConnectionManager) connect() error {
	conn, err := amqp.Dial(m.url)
	if err != nil {
		return err
	}
	m.setConnection(conn)
	return nil
}

func (m *ConnectionManager) GetChannel() (*amqp.Channel, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.connection == nil {
		return nil, errors.New("connection is not established")
	}

	return m.connection.Channel()
}

func (m *ConnectionManager) isConnected() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.connection != nil && !m.connection.IsClosed()
}

func (m *ConnectionManager) setConnection(conn *amqp.Connection) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.connection = conn
}

func (m *ConnectionManager) Close() {
	m.once.Do(
		func() {
			close(m.done)
			if m.isConnected() {
				m.logger.Info("Closing RabbitMQ connection")
				if err := m.connection.Close(); err != nil {
					m.logger.Error("Failed to close connection", "error", err)
				}
			}
		}
	)
}

func (m *ConnectionManager) WaitUntilReady() {
	<-m.ready
}