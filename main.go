package main

import (
	"io"
	"log"
	"net/http"
	"os"
)

type AppLogger struct {
	logFile *os.File
}

func NewAppLogger(filename string) *AppLogger {
	file, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	return &AppLogger{logFile: file}
}

func (l *AppLogger) Log(message string) {
	l.logFile.WriteString(message + "\n")
	log.Println(message)
}

func main() {
	logger := NewAppLogger("llm.log")
	defer logger.logFile.Close()

	http.HandleFunc("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		// Log the request
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Log("模型请求：" + string(body))

		// Create new request to OpenRouter
		req, err := http.NewRequest("POST", "https://api.deepseek.com/chat/completions", r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Copy headers
		req.Header = r.Header.Clone()
		req.Header.Set("Accept", "text/event-stream")

		// Make the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Stream the response line by line
		w.Header().Set("Content-Type", "text/event-stream")
		logger.Log("模型返回：")

		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				logger.Log(string(buf[:n]))
			}
			if err != nil {
				if err != io.EOF {
					logger.Log("Stream error: " + err.Error())
				}
				break
			}
		}
	})

	log.Fatal(http.ListenAndServe(":8000", nil))
}
