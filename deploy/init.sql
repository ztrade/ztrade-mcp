CREATE DATABASE IF NOT EXISTS exchange CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE exchange;

-- Strategy scripts
CREATE TABLE IF NOT EXISTS mcp_scripts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description VARCHAR(500) DEFAULT '',
    content LONGTEXT NOT NULL,
    language VARCHAR(20) DEFAULT 'go',
    tags VARCHAR(500) DEFAULT '',
    status VARCHAR(20) DEFAULT 'active',
    lifecycle_status VARCHAR(20) DEFAULT 'research',
    field_descriptions TEXT DEFAULT NULL,
    version INT DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_status (status),
    INDEX idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Script version history
CREATE TABLE IF NOT EXISTS mcp_script_versions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    script_id BIGINT NOT NULL,
    version INT NOT NULL,
    content LONGTEXT NOT NULL,
    message VARCHAR(500) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_script_id (script_id),
    INDEX idx_script_version (script_id, version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Backtest records
CREATE TABLE IF NOT EXISTS mcp_backtest_records (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    script_id BIGINT NOT NULL,
    script_version INT NOT NULL,
    exchange VARCHAR(50) NOT NULL,
    symbol VARCHAR(50) NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    init_balance DOUBLE DEFAULT 0,
    fee DOUBLE DEFAULT 0,
    lever DOUBLE DEFAULT 1,
    param TEXT DEFAULT NULL,
    total_actions INT DEFAULT 0,
    win_rate DOUBLE DEFAULT 0,
    total_profit DOUBLE DEFAULT 0,
    profit_percent DOUBLE DEFAULT 0,
    max_drawdown DOUBLE DEFAULT 0,
    max_drawdown_value DOUBLE DEFAULT 0,
    max_lose DOUBLE DEFAULT 0,
    total_fee DOUBLE DEFAULT 0,
    start_balance DOUBLE DEFAULT 0,
    end_balance DOUBLE DEFAULT 0,
    total_return DOUBLE DEFAULT 0,
    annual_return DOUBLE DEFAULT 0,
    sharpe_ratio DOUBLE DEFAULT 0,
    sortino_ratio DOUBLE DEFAULT 0,
    volatility DOUBLE DEFAULT 0,
    profit_factor DOUBLE DEFAULT 0,
    calmar_ratio DOUBLE DEFAULT 0,
    overall_score DOUBLE DEFAULT 0,
    long_trades INT DEFAULT 0,
    short_trades INT DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_script_id (script_id),
    INDEX idx_script_version (script_id, script_version),
    INDEX idx_overall_score (overall_score)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Backtest logs (captured engine.Log output)
CREATE TABLE IF NOT EXISTS mcp_backtest_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    record_id BIGINT NOT NULL,
    line_no INT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_record_id (record_id),
    INDEX idx_record_line (record_id, line_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
