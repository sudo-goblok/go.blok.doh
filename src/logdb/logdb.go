package logdb

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// Struktur log DNS
type DNSLog struct {
	Timestamp   int64  `json:"timestamp"` // Unix timestamp nanodetik
	ClientIP    string `json:"client_ip"`
	Query       string `json:"query"`
	QueryType   int    `json:"query_type"`
	Resolver    string `json:"resolver"`
	ResolverURL string `json:"resolver_url"`
	Response    []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		Class int	`json:"class"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"response"`
	Comment []string `json:"comment"`
}

// LogManager untuk mengelola penyimpanan log
type LogManager struct {
	DB *badger.DB
}

// NewLogManager membuka database dan mengembalikan instance LogManager
func NewLogManager(dbPath string) (*LogManager, error) {
	// Pastikan folder database ada
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		os.MkdirAll(dbPath, 0755)
	}

	// Buka BadgerDB dengan opsi optimal untuk log
	opts := badger.DefaultOptions(dbPath).
		WithLogger(nil).      // Nonaktifkan logging internal Badger
		WithSyncWrites(false) // Performa write lebih cepat tanpa sync tiap transaksi

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &LogManager{DB: db}, nil
}

// Close menutup koneksi ke BadgerDB
func (lm *LogManager) Close() {
	lm.DB.Close()
}

// SaveLog menyimpan log ke BadgerDB
func (lm *LogManager) SaveLog(logEntry DNSLog) error {
	logEntry.Timestamp = time.Now().UnixNano() // Tambahkan timestamp jika belum ada
	data, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}

	// Key berbasis timestamp untuk memudahkan pencarian berdasarkan waktu
	key := fmt.Sprintf("%d_%s_%s", logEntry.Timestamp, logEntry.ClientIP, logEntry.Query)

	return lm.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})
}

// ReadLogs membaca semua log dari BadgerDB
func (lm *LogManager) ReadLogs() ([]DNSLog, error) {
	var logs []DNSLog

	err := lm.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // Optimasi: hanya ambil key dulu, value diambil saat perlu
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var logEntry DNSLog
				if err := json.Unmarshal(val, &logEntry); err == nil {
					logs = append(logs, logEntry)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return logs, err
}

// ReadLogsByRange membaca log dalam rentang waktu tertentu
func (lm *LogManager) ReadLogsByRange(startTime, endTime int64) ([]DNSLog, error) {
	var logs []DNSLog

	err := lm.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		startKey := fmt.Sprintf("%d_", startTime) // Key awal dalam rentang
		endKey := fmt.Sprintf("%d_", endTime)     // Key akhir dalam rentang

		for it.Seek([]byte(startKey)); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())

			// Stop jika sudah melewati endTime
			if key > endKey {
				break
			}

			err := item.Value(func(val []byte) error {
				var logEntry DNSLog
				if err := json.Unmarshal(val, &logEntry); err == nil {
					logs = append(logs, logEntry)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return logs, err
}

// DeleteOldLogs menghapus log yang lebih lama dari batas waktu tertentu
func (lm *LogManager) DeleteOldLogs(beforeTime int64) error {
	return lm.DB.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := fmt.Sprintf("%d_", beforeTime)
		for it.Seek([]byte(prefix)); it.Valid(); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)

			// Jika key lebih kecil dari beforeTime, hapus
			if string(key) < prefix {
				if err := txn.Delete(key); err != nil {
					return err
				}
			} else {
				break
			}
		}
		return nil
	})
}
