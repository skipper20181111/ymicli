package usageSave

import (
	"fmt"
	"time"
)

func TestInsertTokenUse() error {
	connector, err := NewMySQLConnector()
	if err != nil {
		return err
	}
	defer connector.Close()

	userSN := fmt.Sprintf("TEST-SN-%d", time.Now().Unix())
	token := int64(2500)

	ip, systemType := GetHostInfo()
	if ip == "" {
		ip = "127.0.0.1"
	}

	err = connector.InsertTokenUse(userSN, token, &ip, &systemType)
	if err != nil {
		return err
	}

	_, err = connector.GetTokenUseByUserSN(userSN, 1)
	if err != nil {
		return err
	}

	return nil
}

func RunTest() {
	TestInsertTokenUse()
}
