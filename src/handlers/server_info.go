package handlers

import (
	"context"

	"bitbucket.org/calmisland/go-server-info/serverinfo"
	"bitbucket.org/calmisland/go-server-requests/apirequests"
)

type serverInfoResponseBody struct {
	RegionName string `json:"region"`
	StageName  string `json:"stage"`
}

// HandleServerInfo handles server information requests.
func HandleServerInfo(ctx context.Context, req *apirequests.Request, resp *apirequests.Response) error {
	serverRegionName := serverinfo.GetRegionName()
	serverStageName := serverinfo.GetStageName()
	response := serverInfoResponseBody{
		RegionName: serverRegionName,
		StageName:  serverStageName,
	}
	resp.SetBody(&response)
	return nil
}
