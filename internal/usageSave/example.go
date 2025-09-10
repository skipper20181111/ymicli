package usageSave

func ExampleUsage() {
	connector, err := NewMySQLConnector()
	if err != nil {
		return
	}
	defer connector.Close()

	userSN := "DESKTOP-ABC123-SN001"
	token := int64(1500)
	ip := "192.168.1.100"
	systemType := "Windows 11"

	err = connector.InsertTokenUse(userSN, token, &ip, &systemType)
	if err != nil {
		return
	}

	_, err = connector.GetTokenUseByUserSN(userSN, 10)
	if err != nil {
		return
	}
}
