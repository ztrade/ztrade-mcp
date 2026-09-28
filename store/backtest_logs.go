package store

import "fmt"

const maxBacktestLogInsertBatchSize = 1000

// SaveBacktestLogs persists captured engine.Log lines for a backtest record.
func (s *Store) SaveBacktestLogs(recordID int64, lines []string) error {
	if recordID <= 0 {
		return fmt.Errorf("invalid record id %d", recordID)
	}
	if len(lines) == 0 {
		return nil
	}

	logs := make([]BacktestLog, 0, len(lines))
	for i, line := range lines {
		logs = append(logs, BacktestLog{
			RecordID: recordID,
			LineNo:   i + 1,
			Content:  line,
		})
	}

	sess := s.engine.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = sess.Rollback()
		}
	}()

	for start := 0; start < len(logs); start += maxBacktestLogInsertBatchSize {
		end := start + maxBacktestLogInsertBatchSize
		if end > len(logs) {
			end = len(logs)
		}
		batch := logs[start:end]
		if _, err := sess.Insert(&batch); err != nil {
			return err
		}
	}

	if err := sess.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// ListBacktestLogs returns paginated captured logs for one backtest record.
func (s *Store) ListBacktestLogs(recordID int64, offset, limit int) ([]BacktestLog, int64, error) {
	if recordID <= 0 {
		return nil, 0, fmt.Errorf("invalid record id %d", recordID)
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	total, err := s.engine.Count(&BacktestLog{RecordID: recordID})
	if err != nil {
		return nil, 0, err
	}

	var logs []BacktestLog
	err = s.engine.Asc("line_no").Limit(limit, offset).Find(&logs, &BacktestLog{RecordID: recordID})
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
