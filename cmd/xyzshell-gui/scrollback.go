package main

import (
	"sync"

	"fyne.io/fyne/v2/widget"
)

const maxPendingScrollbackRows = 2048

// scrollbackRowQueue hands copied terminal rows from the output reader to the
// GUI in batches, keeping terminal output independent of UI render speed.
type scrollbackRowQueue struct {
	mu      sync.Mutex
	rows    []widget.TextGridRow
	dropped int
}

func (q *scrollbackRowQueue) Append(row widget.TextGridRow) {
	row.Cells = append([]widget.TextGridCell(nil), row.Cells...)
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.rows) >= maxPendingScrollbackRows {
		copy(q.rows, q.rows[1:])
		q.rows[len(q.rows)-1] = row
		q.dropped++
		return
	}
	q.rows = append(q.rows, row)
}

func (q *scrollbackRowQueue) Drain() ([]widget.TextGridRow, int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	rows, dropped := q.rows, q.dropped
	q.rows = nil
	q.dropped = 0
	return rows, dropped
}

func (q *scrollbackRowQueue) Reset() {
	q.mu.Lock()
	q.rows = nil
	q.dropped = 0
	q.mu.Unlock()
}
