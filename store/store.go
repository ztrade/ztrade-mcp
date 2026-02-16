package store

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"xorm.io/xorm"
)

// Script represents a strategy script stored in the database.
type Script struct {
	ID          int64     `xorm:"pk autoincr" json:"id"`
	Name        string    `xorm:"varchar(100) notnull unique" json:"name"`
	Description string    `xorm:"varchar(500)" json:"description"`
	Content     string    `xorm:"longtext notnull" json:"content"`
	Language    string    `xorm:"varchar(20) default('go')" json:"language"`
	Tags        string    `xorm:"varchar(500)" json:"tags"`
	Status      string    `xorm:"varchar(20) default('active')" json:"status"` // active, archived, deleted
	Version     int       `xorm:"default(1)" json:"version"`
	CreatedAt   time.Time `xorm:"created" json:"createdAt"`
	UpdatedAt   time.Time `xorm:"updated" json:"updatedAt"`
}

func (Script) TableName() string {
	return "mcp_scripts"
}

// ScriptVersion represents a historical version of a script.
type ScriptVersion struct {
	ID        int64     `xorm:"pk autoincr" json:"id"`
	ScriptID  int64     `xorm:"notnull index" json:"scriptId"`
	Version   int       `xorm:"notnull" json:"version"`
	Content   string    `xorm:"longtext notnull" json:"content"`
	Message   string    `xorm:"varchar(500)" json:"message"`
	CreatedAt time.Time `xorm:"created" json:"createdAt"`
}

func (ScriptVersion) TableName() string {
	return "mcp_script_versions"
}

// BacktestRecord represents a backtest result for a script.
type BacktestRecord struct {
	ID               int64     `xorm:"pk autoincr" json:"id"`
	ScriptID         int64     `xorm:"notnull index" json:"scriptId"`
	ScriptVersion    int       `xorm:"notnull" json:"scriptVersion"`
	Exchange         string    `xorm:"varchar(50) notnull" json:"exchange"`
	Symbol           string    `xorm:"varchar(50) notnull" json:"symbol"`
	StartTime        time.Time `xorm:"notnull" json:"startTime"`
	EndTime          time.Time `xorm:"notnull" json:"endTime"`
	InitBalance      float64   `json:"initBalance"`
	Fee              float64   `json:"fee"`
	Lever            float64   `json:"lever"`
	Param            string    `xorm:"text" json:"param"`
	TotalActions     int       `json:"totalActions"`
	WinRate          float64   `json:"winRate"`
	TotalProfit      float64   `json:"totalProfit"`
	ProfitPercent    float64   `json:"profitPercent"`
	MaxDrawdown      float64   `json:"maxDrawdown"`
	MaxDrawdownValue float64   `json:"maxDrawdownValue"`
	MaxLose          float64   `json:"maxLose"`
	TotalFee         float64   `json:"totalFee"`
	StartBalance     float64   `json:"startBalance"`
	EndBalance       float64   `json:"endBalance"`
	TotalReturn      float64   `json:"totalReturn"`
	AnnualReturn     float64   `json:"annualReturn"`
	SharpeRatio      float64   `json:"sharpeRatio"`
	SortinoRatio     float64   `json:"sortinoRatio"`
	Volatility       float64   `json:"volatility"`
	ProfitFactor     float64   `json:"profitFactor"`
	CalmarRatio      float64   `json:"calmarRatio"`
	OverallScore     float64   `json:"overallScore"`
	LongTrades       int       `json:"longTrades"`
	ShortTrades      int       `json:"shortTrades"`
	CreatedAt        time.Time `xorm:"created" json:"createdAt"`
}

func (BacktestRecord) TableName() string {
	return "mcp_backtest_records"
}

// Store provides database operations for script management.
type Store struct {
	engine *xorm.Engine
}

// NewStore creates a new Store from viper config.
func NewStore(cfg *viper.Viper) (*Store, error) {
	dbType := cfg.GetString("db.type")
	dbURI := cfg.GetString("db.uri")
	if dbType == "" || dbURI == "" {
		return nil, fmt.Errorf("db.type and db.uri must be configured")
	}

	engine, err := xorm.NewEngine(dbType, dbURI)
	if err != nil {
		return nil, fmt.Errorf("failed to create db engine: %w", err)
	}

	// Auto-sync tables
	if err := engine.Sync2(new(Script), new(ScriptVersion), new(BacktestRecord)); err != nil {
		return nil, fmt.Errorf("failed to sync tables: %w", err)
	}

	log.Info("Script store initialized")
	return &Store{engine: engine}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.engine.Close()
}

// --- Script CRUD ---

// CreateScript creates a new script and saves its initial version.
func (s *Store) CreateScript(script *Script) error {
	script.Version = 1
	script.Status = "active"
	_, err := s.engine.Insert(script)
	if err != nil {
		return err
	}
	// Save initial version
	ver := &ScriptVersion{
		ScriptID: script.ID,
		Version:  1,
		Content:  script.Content,
		Message:  "initial version",
	}
	_, err = s.engine.Insert(ver)
	return err
}

// GetScript retrieves a script by ID.
func (s *Store) GetScript(id int64) (*Script, error) {
	script := &Script{}
	has, err := s.engine.ID(id).Get(script)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, fmt.Errorf("script with id %d not found", id)
	}
	return script, nil
}

// GetScriptByName retrieves a script by name.
func (s *Store) GetScriptByName(name string) (*Script, error) {
	script := &Script{}
	has, err := s.engine.Where("name = ?", name).Get(script)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, fmt.Errorf("script '%s' not found", name)
	}
	return script, nil
}

// ListScripts lists scripts with optional filters.
func (s *Store) ListScripts(status, keyword string) ([]Script, error) {
	var scripts []Script
	sess := s.engine.NewSession()
	defer sess.Close()

	if status != "" {
		sess = sess.Where("status = ?", status)
	} else {
		sess = sess.Where("status != ?", "deleted")
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		sess = sess.Where("(name LIKE ? OR description LIKE ? OR tags LIKE ?)", like, like, like)
	}
	err := sess.OrderBy("updated_at DESC").Find(&scripts)
	return scripts, err
}

// UpdateScript updates a script's content and bumps the version.
func (s *Store) UpdateScript(id int64, content, message string) (*Script, error) {
	script, err := s.GetScript(id)
	if err != nil {
		return nil, err
	}

	script.Version++
	script.Content = content

	_, err = s.engine.ID(id).Cols("content", "version", "updated_at").Update(script)
	if err != nil {
		return nil, err
	}

	// Save version history
	ver := &ScriptVersion{
		ScriptID: id,
		Version:  script.Version,
		Content:  content,
		Message:  message,
	}
	_, err = s.engine.Insert(ver)
	if err != nil {
		return nil, err
	}

	return script, nil
}

// UpdateScriptMeta updates script metadata (name, description, tags, status).
func (s *Store) UpdateScriptMeta(id int64, fields map[string]interface{}) error {
	_, err := s.engine.Table(new(Script)).ID(id).Update(fields)
	return err
}

// DeleteScript soft-deletes a script by setting status to "deleted".
func (s *Store) DeleteScript(id int64) error {
	_, err := s.engine.ID(id).Cols("status").Update(&Script{Status: "deleted"})
	return err
}

// --- Version Management ---

// ListVersions lists all versions of a script.
func (s *Store) ListVersions(scriptID int64) ([]ScriptVersion, error) {
	var versions []ScriptVersion
	err := s.engine.Where("script_id = ?", scriptID).OrderBy("version DESC").Find(&versions)
	return versions, err
}

// GetVersion retrieves a specific version of a script.
func (s *Store) GetVersion(scriptID int64, version int) (*ScriptVersion, error) {
	ver := &ScriptVersion{}
	has, err := s.engine.Where("script_id = ? AND version = ?", scriptID, version).Get(ver)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, fmt.Errorf("version %d of script %d not found", version, scriptID)
	}
	return ver, nil
}

// RollbackScript reverts a script to a specific version.
func (s *Store) RollbackScript(scriptID int64, version int) (*Script, error) {
	ver, err := s.GetVersion(scriptID, version)
	if err != nil {
		return nil, err
	}
	return s.UpdateScript(scriptID, ver.Content, fmt.Sprintf("rollback to version %d", version))
}

// DiffVersions returns content of two versions for comparison.
func (s *Store) DiffVersions(scriptID int64, v1, v2 int) (*ScriptVersion, *ScriptVersion, error) {
	ver1, err := s.GetVersion(scriptID, v1)
	if err != nil {
		return nil, nil, fmt.Errorf("version %d: %w", v1, err)
	}
	ver2, err := s.GetVersion(scriptID, v2)
	if err != nil {
		return nil, nil, fmt.Errorf("version %d: %w", v2, err)
	}
	return ver1, ver2, nil
}

// --- Backtest Records ---

// SaveBacktestRecord saves a backtest result.
func (s *Store) SaveBacktestRecord(record *BacktestRecord) error {
	if record == nil {
		return fmt.Errorf("backtest record is nil")
	}
	if fields := sanitizeBacktestRecordForInsert(record); len(fields) > 0 {
		log.WithField("fields", fields).Warn("sanitized non-finite backtest record fields")
	}
	_, err := s.engine.Insert(record)
	return err
}

// ListBacktestRecords lists backtest records for a script.
func (s *Store) ListBacktestRecords(scriptID int64, limit int) ([]BacktestRecord, error) {
	var records []BacktestRecord
	sess := s.engine.Where("script_id = ?", scriptID).OrderBy("created_at DESC")
	if limit > 0 {
		sess = sess.Limit(limit)
	}
	err := sess.Find(&records)
	return records, err
}

// GetBestBacktest returns the best performing backtest for a script by overall score.
func (s *Store) GetBestBacktest(scriptID int64) (*BacktestRecord, error) {
	record := &BacktestRecord{}
	has, err := s.engine.Where("script_id = ?", scriptID).OrderBy("overall_score DESC").Get(record)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, fmt.Errorf("no backtest records found for script %d", scriptID)
	}
	return record, nil
}

// GetBacktestSummary returns aggregate stats for a script's backtest history.
func (s *Store) GetBacktestSummary(scriptID int64) (map[string]interface{}, error) {
	records, err := s.ListBacktestRecords(scriptID, 0)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no backtest records found for script %d", scriptID)
	}

	var totalScore, bestScore, worstScore float64
	var bestSharpe, worstSharpe float64
	var bestWinRate, worstWinRate float64
	var bestRecord, worstRecord *BacktestRecord

	worstScore = 1e18
	worstSharpe = 1e18
	worstWinRate = 1e18

	for i := range records {
		r := &records[i]
		totalScore += r.OverallScore
		if r.OverallScore > bestScore {
			bestScore = r.OverallScore
			bestRecord = r
		}
		if r.OverallScore < worstScore {
			worstScore = r.OverallScore
			worstRecord = r
		}
		if r.SharpeRatio > bestSharpe {
			bestSharpe = r.SharpeRatio
		}
		if r.SharpeRatio < worstSharpe {
			worstSharpe = r.SharpeRatio
		}
		if r.WinRate > bestWinRate {
			bestWinRate = r.WinRate
		}
		if r.WinRate < worstWinRate {
			worstWinRate = r.WinRate
		}
	}

	summary := map[string]interface{}{
		"totalRuns":    len(records),
		"avgScore":     totalScore / float64(len(records)),
		"bestScore":    bestScore,
		"worstScore":   worstScore,
		"bestSharpe":   bestSharpe,
		"worstSharpe":  worstSharpe,
		"bestWinRate":  bestWinRate,
		"worstWinRate": worstWinRate,
	}
	if bestRecord != nil {
		summary["bestRun"] = map[string]interface{}{
			"id":       bestRecord.ID,
			"version":  bestRecord.ScriptVersion,
			"exchange": bestRecord.Exchange,
			"symbol":   bestRecord.Symbol,
			"param":    bestRecord.Param,
		}
	}
	if worstRecord != nil {
		summary["worstRun"] = map[string]interface{}{
			"id":       worstRecord.ID,
			"version":  worstRecord.ScriptVersion,
			"exchange": worstRecord.Exchange,
			"symbol":   worstRecord.Symbol,
			"param":    worstRecord.Param,
		}
	}
	return summary, nil
}
