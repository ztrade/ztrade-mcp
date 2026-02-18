package store

import "fmt"

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
	_, err := s.engine.Insert(&logs)
	return err
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

	total, err := s.engine.Where("record_id = ?", recordID).Count(new(BacktestLog))
	if err != nil {
		return nil, 0, err
	}

	var logs []BacktestLog
	err = s.engine.Where("record_id = ?", recordID).Asc("line_no").Limit(limit, offset).Find(&logs)
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
