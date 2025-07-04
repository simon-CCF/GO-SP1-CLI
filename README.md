這是一個使用 Golang 編寫的命令列日誌處理工具，具備以下功能：

- 每秒產生大量模擬 JSON 格式日誌
- 自動將 `ERROR` 和 `CRITICAL` 等級的日誌寫入 SQLite 資料庫
- 實作可動態擴縮的 worker pool（使用 context 控制 goroutine）
- 根據 log channel 的長度，自動增減工作 goroutine 數量
- 每 15 秒列出最近 5 筆錯誤資訊

## 技術重點

- 使用 `context.WithCancel()` 控制每個 worker 的生命週期
- `sync.WaitGroup` 保證 goroutine 正確結束
- SQLite 搭配 `WAL 模式` 提升寫入效能
- 動態 worker 池（auto scale worker pool）
- 使用 channel 串接資料處理流程
