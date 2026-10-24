package data

import (
	"errors"
	"fmt"

	"github.com/reactor2k3/ddns-updater/internal/records"
)

var ErrRecordNotFound = errors.New("record not found")

func (db *Database) Select(id uint) (record records.Record, err error) {
	db.RLock()
	defer db.RUnlock()
	if id >= uint(len(db.data)) {
		return record, fmt.Errorf("%w: for id %d", ErrRecordNotFound, id)
	}
	record = db.data[id]
	record.History = append(record.History[:0:0], record.History...)
	return record, nil
}

func (db *Database) SelectAll() []records.Record {
	db.RLock()
	defer db.RUnlock()
	result := append([]records.Record(nil), db.data...)
	for i := range result {
		result[i].History = append(result[i].History[:0:0], result[i].History...)
	}
	return result
}

// Replace runs inside the update service's transaction queue.
func (db *Database) Replace(next []records.Record) {
	db.Lock()
	defer db.Unlock()
	db.data = next
}
