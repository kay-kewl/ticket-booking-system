package rabbitmq

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ConnectionManager struct {
	logger		*slog.Logger
	url			string
	connection 	*amqp.Connection
	mutex		sync.RWMutex
	ready		chan bool
	done		chan bool
}

func NewConnectionManager(url string, logger *slog.Logger) *ConnectionManager {
	m := &ConnectionManager{
		logger: 	logger.With(slog.String("component", "RabbitMQManager")),
		url:		url,
		ready:		make(chan bool),
		done:		make(chan bool),
	}

	go m.handleReconnect()
	return m
}

func (m *ConnectionManager) handleReconnect() {
	m.logger.Info("Connection manager started")

	for {
		select {
		case <-m.done:
			m.logger.Info("Connection manager stopped")
			return
		default:
			if m.isConnected() {
				m.logger.Info("Attempting to connect to RabbitMQ...")
				if err := m.connect(); err != nil {
					m.logger.Error("Failed to connect, retrying...", "error", err)
					time.Sleep(60 * time.Second)
					continue
				}
				m.logger.Info("Connection established!")
				select {
				case m.ready <- true:
				default:
				}
			}
		}

		if m.isConnected() {
			notifyClose := make(chan *amqp.Error)
			m.connection.NotifyClose(notifyClose)

			select {
			case <-m.done:
				m.logger.Info("Connection manager stopping while connected")
				m.Close()
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
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.connection == nil {
		return nil, errors.New("connection is not established")
	}

	return m.connection.Channel()
}

func (m *ConnectionManager) isConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.connection != nil && !m.connection.IsClosed()
}

func (m *ConnectionManager) setConnection(conn *amqp.Connection) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	m.connection = conn
}

func (m *ConnectionManager) Close() {
	close(m.done)
	if m.isConnected() {
		m.logger.Info("Closing RabbitMQ connection")
		if err := m.connection.Close(); err != nil {
			m.logger.Error("Failed to close connection", "error", err)
		}
	}
}

func (m *ConnectionManager) WaitUntilReady() {
	<-m.ready
}