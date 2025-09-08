/*
ВАЖНО: Этот middleware отключен и не должен использоваться в продакшене в текущем виде.
Проблемы:
1.  Хранение ключей в памяти (in-memory map) приводит к их потере при перезапуске.
2.  Возможна утечка памяти при большом количестве запросов.
3.  Есть состояние гонки (race condition) при проверке и установке ключа.
4.  Не реализовано кэширование ответа.

Для правильной реализации следует использовать внешнее хранилище, такое как Redis,
с атомарной операцией SETNX для ключа и хранением ответа.
*/
package middleware

import (
	"net/http"
	"sync"
	"time"
)

var (
	requests 		= make(map[string]time.Time)
	requestsMutex 	= &sync.Mutex{}
	ttl				= 15 * time.Minute
)

func Idempotency(next http.Handler) http.Handler {
	go func() {
		for range time.Tick(1 * time.Minute) {
			requestsMutex.Lock()
			for k, v := range requests {
				if time.Since(v) > ttl {
					delete(requests, k)
				}
			}
			requestsMutex.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		requestsMutex.Lock()
		if _, found := requests[key]; found {
			requestsMutex.Unlock()
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte("Request with this Idempotency-Key already processed"))
			return
		}

		requests[key] = time.Now()
		requestsMutex.Unlock()

		next.ServeHTTP(w, r)
	})
}