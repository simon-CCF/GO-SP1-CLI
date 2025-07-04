package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"
)

const YMDHMS = "2006-01-02 15:04:05"

type Log struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Time    string `json:"time"`
	Source  string `json:"source"`
}

var levels = []string{"debug", "info", "error", "warning"}
var messages = []string{
	"User login success",
	"File not found",
	"Database connection failed",
	"Cache miss",
	"Timeout error",
}
var sources = []string{"auth-service", "db-service", "cache-service", "api-gateway"}

func fakelog(ch chan<- Log, n int) {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		<-timer.C
		for i := 0; i < n; i++ {
			log := Log{
				Level:   levels[rand.Intn(len(levels))],
				Message: messages[rand.Intn(len(messages))],
				Time:    time.Now().Format(YMDHMS),
				Source:  sources[rand.Intn(len(sources))],
			}
			ch <- log
		}
	}
}

func main() {
	db, err := sql.Open("sqlite3", "./logs.db")
	if err != nil {
		log.Fatal(err)
	}
	sqltable := `CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		level TEXT,
		message TEXT,
		timestamp TEXT,
		source TEXT
	);`
	search := `SELECT level, message, timestamp, source FROM logs ORDER BY id DESC LIMIT 5`
	_, err = db.Exec(sqltable)
	if err != nil {
		log.Fatal(err)
	}
	ch1 := make(chan Log, 100)
	alerttimer := time.NewTicker(20 * time.Second)
	defer alerttimer.Stop()
	go fakelog(ch1, 3)
	go func() {
		for logEntry := range ch1 {
			// 只存 ERROR 跟 CRITICAL
			if logEntry.Level == "ERROR" || logEntry.Level == "CRITICAL" {
				_, err := db.Exec(
					"INSERT INTO logs(level, message, timestamp, source) VALUES (?, ?, ?, ?)",
					logEntry.Level, logEntry.Message, logEntry.Time, logEntry.Source,
				)
				if err != nil {
					fmt.Println("DB insert error:", err)
				}
			}
		}
	}()
	errcount := 0
	errnotice := 5
	for {
		select {
		case <-alerttimer.C:
			if errcount > errnotice {
				fmt.Printf("🚨 告警：過去 20 秒內有 %d 筆 ERROR 訊息！\n", errcount)
			}
			errcount = 0
		case result := <-ch1:
			if result.Level == "error" {
				errcount++
				json1, err := json.Marshal(result)
				if err != nil {
					fmt.Println(err)
					continue
				}
				fmt.Println(string(json1))
			}

		}

	}

	// for result := range ch1 {
	// 	json1, err := json.Marshal(result)
	// 	if err != nil {
	// 		fmt.Println(err)
	// 		continue
	// 	}
	// 	if result.Level == "error" {
	// 		errcount++
	// 		fmt.Println(string(json1))
	// 	}
	// 	if errcount > errnotice {
	// 		fmt.Printf("🚨 告警：過去 10 秒內有 %d 筆 ERROR 訊息！\n", errcount)
	// 	}
	// 	select {
	// 	case <-alerttimer.C:
	// 		errcount = 0
	// 	}
	// }
}
