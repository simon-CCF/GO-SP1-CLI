package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	_ "modernc.org/sqlite" // 驅動
)

type Log struct {
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
}

var (
	levels       = []string{"DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"}
	messages     = []string{"User login success", "File not found", "Database connection failed", "Cache miss", "Timeout error"}
	sources      = []string{"auth-service", "db-service", "cache-service", "api-gateway"}
	mu           sync.Mutex
	workers      = make(map[int]context.CancelFunc)
	nextWorkerID = 1
	wg           sync.WaitGroup

	logCh = make(chan Log, 100) // 全域 logCh
)

func logGenerator(ch chan<- Log, n int) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		for i := 0; i < n; i++ {
			log := Log{
				Level:     levels[rand.Intn(len(levels))],
				Message:   messages[rand.Intn(len(messages))],
				Timestamp: time.Now().Format(time.RFC3339),
				Source:    sources[rand.Intn(len(sources))],
			}
			ch <- log
		}
	}
}

func worker(ctx context.Context, id int, ch <-chan Log, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Worker %d stopped\n", id)
			return
		case logEntry, ok := <-ch:
			if !ok {
				fmt.Printf("Worker %d: channel closed\n", id)
				return
			}
			jsonBytes, err := json.Marshal(logEntry)
			if err != nil {
				fmt.Printf("Worker %d - JSON marshal error: %v\n", id, err)
				continue
			}
			fmt.Printf("Worker %d - %s\n", id, string(jsonBytes))
		}
	}
}

func adjustWorkers(target int) {
	mu.Lock()
	defer mu.Unlock()
	current := len(workers)
	if target > current {
		for i := 0; i < target-current; i++ {
			ctx, cancel := context.WithCancel(context.Background())
			id := nextWorkerID
			nextWorkerID++
			wg.Add(1)
			go worker(ctx, id, logCh, &wg)
			workers[id] = cancel
			fmt.Printf("Added worker %d\n", id)
		}
	} else if target < current {
		removeCount := current - target
		for id, cancel := range workers {
			if removeCount == 0 {
				break
			}
			cancel()
			delete(workers, id)
			removeCount--
			fmt.Printf("Removed worker %d\n", id)
		}
	}
}

func main() {
	db, err := sql.Open("sqlite", "./logs.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("PRAGMA journal_mode = WAL;")
	if err != nil {
		log.Fatal("Failed to enable WAL mode:", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		level TEXT,
		message TEXT,
		timestamp TEXT,
		source TEXT
	);
	`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatal(err)
	}

	go logGenerator(logCh, 100)
	adjustWorkers(2) // 一開始開 2 個工人

	// 寫入資料庫 goroutine
	go func() {
		for logEntry := range logCh {
			if logEntry.Level == "ERROR" || logEntry.Level == "CRITICAL" {
				_, err := db.Exec(
					"INSERT INTO logs(level, message, timestamp, source) VALUES (?, ?, ?, ?)",
					logEntry.Level, logEntry.Message, logEntry.Timestamp, logEntry.Source,
				)
				if err != nil {
					fmt.Println("DB insert error:", err)
				}
			}
		}
	}()

	// ======= 新增：根據 logCh 長度動態調整工人數 =======
	go func() {
		for {
			time.Sleep(1 * time.Second)
			queueLen := len(logCh)
			fmt.Printf("Queue length: %d, workers: %d\n", queueLen, len(workers))

			if queueLen > 50 {
				// 工作太多，增加工人數，最多加到 20 人
				if len(workers) < 20 {
					fmt.Println("⚠️⚠️⚠️⚠️⚠️ Too many logs, adding workers")
					adjustWorkers(len(workers) + 2)
				}
			} else if queueLen < 20 {
				// 工作太少，減少工人數，至少留 2 人
				if len(workers) > 2 {
					fmt.Println("⚠️ Too few logs, reducing workers")
					adjustWorkers(len(workers) - 1)
				}
			}
		}
	}()
	// ========================================================

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Println("===== 最近 5 筆錯誤 =====")
		rows, err := db.Query("SELECT level, message, timestamp, source FROM logs ORDER BY id DESC LIMIT 5")
		if err != nil {
			fmt.Println("DB query error:", err)
			continue
		}

		for rows.Next() {
			var level, message, timestamp, source string
			err = rows.Scan(&level, &message, &timestamp, &source)
			if err != nil {
				fmt.Println("DB scan error:", err)
				continue
			}
			fmt.Printf("[%s] %s - %s (%s)\n", timestamp, level, message, source)
		}
		rows.Close()
	}

	wg.Wait() // 等待所有工人結束（實際上程式永遠不會到這）
}

