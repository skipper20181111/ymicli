package utilFunction

type UserInfoReq struct {
	Userid   string `json:"userid"`
	Language string `json:"language"`
}
type DownLoadDocInitResp struct {
	Result struct {
		TaskId int64 `json:"taskId"`
	} `json:"result"`
	Success bool `json:"success"`
}
type UserInfoResponse struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
	Result  struct {
		Active        bool   `json:"active"`
		Admin         bool   `json:"admin"`
		Avatar        string `json:"avatar"`
		Boss          bool   `json:"boss"`
		CreateTime    string `json:"create_time"`
		DeptIDList    []int  `json:"dept_id_list"`
		DeptOrderList []struct {
			DeptID int   `json:"dept_id"`
			Order  int64 `json:"order"`
		} `json:"dept_order_list"`
		DisableStatus            bool   `json:"disable_status"`
		ExclusiveAccount         bool   `json:"exclusive_account"`
		ExclusiveAccountCorpID   string `json:"exclusive_account_corp_id"`
		ExclusiveAccountCorpName string `json:"exclusive_account_corp_name"`
		ExclusiveAccountType     string `json:"exclusive_account_type"`
		HideMobile               bool   `json:"hide_mobile"`
		HiredDate                int64  `json:"hired_date"`
		JobNumber                string `json:"job_number"`
		LeaderInDept             []struct {
			DeptID int  `json:"dept_id"`
			Leader bool `json:"leader"`
		} `json:"leader_in_dept"`
		LoginID       string `json:"login_id"`
		ManagerUserid string `json:"manager_userid"`
		Name          string `json:"name"`
		Nickname      string `json:"nickname"`
		RealAuthed    bool   `json:"real_authed"`
		RoleList      []struct {
			GroupName string `json:"group_name"`
			ID        int    `json:"id"`
			Name      string `json:"name"`
		} `json:"role_list"`
		Senior  bool   `json:"senior"`
		Title   string `json:"title"`
		Unionid string `json:"unionid"`
		Userid  string `json:"userid"`
	} `json:"result"`
	RequestID string `json:"request_id"`
}
