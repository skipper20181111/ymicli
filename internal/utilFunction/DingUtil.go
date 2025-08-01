package utilFunction

import (
	"bytes"
	"encoding/json"
	"fmt"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dingtalkoauth2_1_0 "github.com/alibabacloud-go/dingtalk/oauth2_1_0"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"io"
	"net/http"
	"os"
)

// DingTalkClient 钉钉API客户端
type DingTalkClient struct {
	AppKey      string
	AppSecret   string
	Client      *dingtalkoauth2_1_0.Client
	AccessToken string
}

// GetAccessToken 获取钉钉应用的access_token
func (d *DingTalkClient) GetAccessToken() (string, error) {
	request := &dingtalkoauth2_1_0.GetAccessTokenRequest{
		AppKey:    tea.String(d.AppKey),
		AppSecret: tea.String(d.AppSecret),
	}

	var accessToken string
	tryErr := func() error {
		defer func() {
			if r := tea.Recover(recover()); r != nil {

			}
		}()

		response, err := d.Client.GetAccessToken(request)
		if err != nil {
			return err
		}
		accessToken = *response.Body.AccessToken
		return nil
	}()

	if tryErr != nil {
		// 处理错误
		var sdkErr = &tea.SDKError{}
		if _t, ok := tryErr.(*tea.SDKError); ok {
			sdkErr = _t
		} else {
			sdkErr.Message = tea.String(tryErr.Error())
		}

		if !tea.BoolValue(util.Empty(sdkErr.Code)) && !tea.BoolValue(util.Empty(sdkErr.Message)) {
			return "", fmt.Errorf("获取access_token失败: [%s] %s", *sdkErr.Code, *sdkErr.Message)
		}
		return "", fmt.Errorf("获取access_token失败: %s", *sdkErr.Message)
	}
	return accessToken, nil
}

// NewDingTalkClient 创建钉钉客户端实例
func NewDingTalkClient() (*DingTalkClient, error) {
	ClientId := "dingew4tmkpzy7vyjdsy"
	ClientSecret := "AP1dzVGdlTa5fq5xu5G4hH7Gfk6tFMeE7tSTbZYqwfW4enSzjFS-wuwE0JmYLbT-"
	config := &openapi.Config{
		Protocol: tea.String("https"),
		RegionId: tea.String("central"),
	}

	client, err := dingtalkoauth2_1_0.NewClient(config)
	if err != nil {
		return nil, err
	}

	Client := &DingTalkClient{
		AppKey:    ClientId,
		AppSecret: ClientSecret,
		Client:    client,
	}
	token, err := Client.GetAccessToken()
	Client.AccessToken = token
	return Client, err
}
func GetAccessToken() string {
	// 创建钉钉客户端
	client, err := NewDingTalkClient()
	if err != nil {

		os.Exit(1)
	}

	// 获取access_token
	token, err := client.GetAccessToken()
	if err != nil {

		os.Exit(1)
	}
	return token
}

func (d *DingTalkClient) GetUserInfo(AccessToken, UserId string) (response UserInfoResponse, ok bool) {
	defer func() {
		if r := recover(); r != nil {

			response = UserInfoResponse{}
			ok = false
		}
	}()
	url := fmt.Sprintf("https://oapi.dingtalk.com/topapi/v2/user/get?access_token=%s", AccessToken)
	requestBody := UserInfoReq{
		Userid:   UserId,
		Language: "zh_CN",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {

		return UserInfoResponse{}, false
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {

		return UserInfoResponse{}, false
	}

	//req.Header.Set("Content-Type", "application/json")
	//req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {

		return UserInfoResponse{}, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {

		return UserInfoResponse{}, false
	}

	if resp.StatusCode != 200 {

		return UserInfoResponse{}, false
	}

	var UserInfoResp UserInfoResponse
	err = json.Unmarshal(body, &UserInfoResp)
	if err != nil {

		return UserInfoResponse{}, false
	}

	return UserInfoResp, true
}
func DownLoadDocInit(AccessToken, NodeId string) (response DownLoadDocInitResp, ok bool) {
	defer func() {
		if r := recover(); r != nil {

			response = DownLoadDocInitResp{}
			ok = false
		}
	}()
	url := fmt.Sprintf("https://api.dingtalk.com/v1.0/doc/%s/export?targetFormat=markdown", NodeId)
	requestBody := UserInfoReq{
		Userid:   "UserId",
		Language: "zh_CN",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {

		return DownLoadDocInitResp{}, false
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {

		return DownLoadDocInitResp{}, false
	}

	//req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {

		return DownLoadDocInitResp{}, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {

		return DownLoadDocInitResp{}, false
	}

	if resp.StatusCode != 200 {

		return DownLoadDocInitResp{}, false
	}

	var downLoadDocInitResp DownLoadDocInitResp
	err = json.Unmarshal(body, &downLoadDocInitResp)
	if err != nil {

		return DownLoadDocInitResp{}, false
	}

	return downLoadDocInitResp, true
}
