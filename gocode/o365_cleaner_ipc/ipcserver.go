package o365_cleaner_ipc

import (
	"cramc_go/common"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
)

func CreateNewEchoServer(hexKey string) (*echo.Echo, error) {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Gzip())
	e.Use(middleware.RemoveTrailingSlash())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	e.GET("/"+hexKey+"/getStatus", progStatusReq)
	e.GET("/"+hexKey+"/pendingFiles", fetchPendingFiles)
	e.POST("/"+hexKey+"/fileHandled", fileHandledLog)
	return e, nil
}

func progStatusReq(c echo.Context) error {
	return c.JSON(http.StatusOK, &common.IPC_StatusResponse{
		Status:              common.RPCHandlingStatus,
		FilesPendingInQueue: len(common.RPCHandlingQueue),
	})
}

func fetchPendingFiles(c echo.Context) error {
	// current supported action: sanitize
	if len(common.RPCHandlingQueue) == 0 {
		return c.String(http.StatusNoContent, "{}")
	} else {
		ipcr := &common.IPC_DocsToBeSanitizedResp{
			Counter:   len(common.RPCHandlingQueue),
			ToProcess: []*common.IPC_SingleDocToBeSanitized{},
		}
		for sdoc := range common.RPCHandlingQueue {
			ipcr.ToProcess = append(ipcr.ToProcess, sdoc)
		}
		return c.JSON(http.StatusOK, ipcr)
	}
}

func fileHandledLog(c echo.Context) error {
	reqBodyObj := new(common.IPC_SanitizedDocsResponse)
	if err := c.Bind(reqBodyObj); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	common.Logger.Infof("Sanitizer report received, processed %d files.\n", reqBodyObj.Counter)
	processedCounter := 0
	for _, v := range reqBodyObj.Processed {
		common.Logger.Infof("From Sanitizer: File %s with Detection %s , Action: %s, Success: %v, Additional Info: %s\n",
			v.Path, v.DetectionName, v.Action, v.IsSuccess, v.AdditionalMsg)
		if !v.IsSuccess {
			common.Logger.Warnln("Sanitizer FAILED due to failure!")
		}
		processedCounter++
	}
	if processedCounter != reqBodyObj.Counter {
		common.Logger.Warnln("Processed Counter does NOT match internal counter, data inconsistent!")
	}
	return c.String(http.StatusOK, "Received!")
}
