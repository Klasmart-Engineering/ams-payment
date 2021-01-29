package v1

import (
	"net/http"

	"bitbucket.org/calmisland/go-server-info/serverinfo"
	"github.com/labstack/echo/v4"
)

type serverInfoResponseBody struct {
	RegionName string `json:"region"`
	StageName  string `json:"stage"`

	// The below ones are for debugging
	ServiceName      string `json:"serviceName,omitempty"`
	BuildDate        string `json:"buildDate,omitempty"`
	BuildReleaseName string `json:"buildReleaseName,omitempty"`
	GitBranch        string `json:"gitBranch,omitempty"`
	GitCommitHash    string `json:"gitCommitHash,omitempty"`
}

// HandleServerInfo handles server information requests.
func HandleServerInfo(c echo.Context) error {
	serverRegionName := serverinfo.GetRegionName()
	serverStageName := serverinfo.GetStageName()
	response := serverInfoResponseBody{
		RegionName: serverRegionName,
		StageName:  serverStageName,
	}

	return c.JSON(http.StatusOK, response)
}
