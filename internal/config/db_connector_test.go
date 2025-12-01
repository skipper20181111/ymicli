package config

import (
	"testing"
)

func TestQueryClaudeConfigByPath(t *testing.T) {
	// 建立数据库连接
	db, err := NewDBConnector()
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 查询 claude_path = "crush" 的配置
	config, err := db.QueryClaudeConfigByPath("crush")
	if err != nil {
		t.Fatalf("Failed to query config: %v", err)
	}

	// 验证结果
	if config == nil {
		t.Fatal("Config is nil")
	}

	t.Logf("ClaudePath: %s", config.ClaudePath)
	if config.Providers != nil {
		t.Logf("Providers: %s", *config.Providers)
	} else {
		t.Log("Providers: NULL")
	}

	// 基本断言
	if config.ClaudePath != "crush" {
		t.Errorf("Expected claude_path 'crush', got '%s'", config.ClaudePath)
	}
}

func TestQueryClaudeConfigByPath_NotFound(t *testing.T) {
	db, err := NewDBConnector()
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 查询不存在的 claude_path
	_, err = db.QueryClaudeConfigByPath("nonexistent_path_12345")
	if err == nil {
		t.Error("Expected error for non-existent claude_path, got nil")
	} else {
		t.Logf("Expected error received: %v", err)
	}
}
