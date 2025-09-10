package usageSave

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type TokenUse struct {
	ID         int64     `db:"id"`
	UserSN     string    `db:"user_sn"`
	Token      int64     `db:"token"`
	CreateTime time.Time `db:"create_time"`
	IP         *string   `db:"ip"`
	SystemType *string   `db:"system_type"`
}

type MySQLConnector struct {
	db *sql.DB
}

func NewMySQLConnector() (*MySQLConnector, error) {
	dsn := "ailaunchcore_qa:xvt8++mN35YwOiLwL2nF@tcp(qa1-mysql.testxinfei.cn:3308)/ailaunchcore?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	return &MySQLConnector{db: db}, nil
}

func (c *MySQLConnector) InsertTokenUse(userSN string, token int64, ip, systemType *string) error {
	query := `
		INSERT INTO token_use (user_sn, token, create_time, ip, system_type) 
		VALUES (?, ?, NOW(), ?, ?)
	`

	_, err := c.db.Exec(query, userSN, token, ip, systemType)
	if err != nil {
		return err
	}

	return nil
}

func (c *MySQLConnector) GetTokenUseByUserSN(userSN string, limit int) ([]TokenUse, error) {
	query := `
		SELECT id, user_sn, token, create_time, ip, system_type 
		FROM token_use 
		WHERE user_sn = ? 
		ORDER BY create_time DESC 
		LIMIT ?
	`

	rows, err := c.db.Query(query, userSN, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TokenUse
	for rows.Next() {
		var tu TokenUse
		err := rows.Scan(&tu.ID, &tu.UserSN, &tu.Token, &tu.CreateTime, &tu.IP, &tu.SystemType)
		if err != nil {
			return nil, err
		}
		results = append(results, tu)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (c *MySQLConnector) InsertTokenUseWithHostInfo(userSN string, token int64) error {
	ip, systemType := GetHostInfo()

	var ipPtr, systemTypePtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if systemType != "" {
		systemTypePtr = &systemType
	}

	return c.InsertTokenUse(userSN, token, ipPtr, systemTypePtr)
}

func (c *MySQLConnector) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}
