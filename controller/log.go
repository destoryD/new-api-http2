package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logs, total, err := model.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func ExportAllLogs(c *gin.Context) {
	query := parseLogExportQuery(c)
	writeLogExport(c, query.Format, func(handle func([]*model.Log) error) error {
		return model.ExportAllLogs(query.LogType, query.StartTimestamp, query.EndTimestamp, query.ModelName, query.Username, query.TokenName, query.Channel, query.Group, query.RequestId, query.UpstreamRequestId, 1000, handle)
	})
}

func ExportUserLogs(c *gin.Context) {
	userId := c.GetInt("id")
	query := parseLogExportQuery(c)
	writeLogExport(c, query.Format, func(handle func([]*model.Log) error) error {
		return model.ExportUserLogs(userId, query.LogType, query.StartTimestamp, query.EndTimestamp, query.ModelName, query.TokenName, query.Group, query.RequestId, query.UpstreamRequestId, 1000, handle)
	})
}

type logExportQuery struct {
	Format            string
	LogType           int
	StartTimestamp    int64
	EndTimestamp      int64
	Username          string
	TokenName         string
	ModelName         string
	Channel           int
	Group             string
	RequestId         string
	UpstreamRequestId string
}

func parseLogExportQuery(c *gin.Context) logExportQuery {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channel, _ := strconv.Atoi(c.Query("channel"))
	format := c.DefaultQuery("format", "csv")
	if format != "txt" {
		format = "csv"
	}
	return logExportQuery{
		Format:            format,
		LogType:           logType,
		StartTimestamp:    startTimestamp,
		EndTimestamp:      endTimestamp,
		Username:          c.Query("username"),
		TokenName:         c.Query("token_name"),
		ModelName:         c.Query("model_name"),
		Channel:           channel,
		Group:             c.Query("group"),
		RequestId:         c.Query("request_id"),
		UpstreamRequestId: c.Query("upstream_request_id"),
	}
}

func writeLogExport(c *gin.Context, format string, export func(func([]*model.Log) error) error) {
	filename := fmt.Sprintf("billing-logs-%s.%s", time.Now().Format("20060102-150405"), format)
	contentType := "text/csv; charset=utf-8"
	if format == "txt" {
		contentType = "text/plain; charset=utf-8"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	writer := csv.NewWriter(c.Writer)
	if format == "txt" {
		writer.Comma = '\t'
	} else {
		_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	}

	if err := writer.Write(logExportHeaders()); err != nil {
		common.ApiError(c, err)
		return
	}

	err := export(func(logs []*model.Log) error {
		for _, log := range logs {
			if err := writer.Write(logExportRow(log)); err != nil {
				return err
			}
		}
		writer.Flush()
		c.Writer.Flush()
		return writer.Error()
	})
	writer.Flush()
	if err == nil {
		err = writer.Error()
	}
	if err != nil {
		common.SysError("failed to export logs: " + err.Error())
	}
}

func logExportHeaders() []string {
	return []string{
		"ID",
		"Time",
		"Type",
		"Username",
		"User ID",
		"Token Name",
		"Token ID",
		"Model Name",
		"Channel ID",
		"Channel",
		"Group",
		"Quota",
		"Prompt Tokens",
		"Completion Tokens",
		"Total Tokens",
		"Duration",
		"Stream",
		"Request ID",
		"Upstream Request ID",
		"IP",
		"Content",
		"Details",
	}
}

func logExportRow(log *model.Log) []string {
	return []string{
		strconv.Itoa(log.Id),
		formatLogExportTime(log.CreatedAt),
		logExportType(log.Type),
		log.Username,
		strconv.Itoa(log.UserId),
		log.TokenName,
		strconv.Itoa(log.TokenId),
		log.ModelName,
		strconv.Itoa(log.ChannelId),
		log.ChannelName,
		log.Group,
		strconv.Itoa(log.Quota),
		strconv.Itoa(log.PromptTokens),
		strconv.Itoa(log.CompletionTokens),
		strconv.Itoa(log.PromptTokens + log.CompletionTokens),
		strconv.Itoa(log.UseTime),
		strconv.FormatBool(log.IsStream),
		log.RequestId,
		log.UpstreamRequestId,
		log.Ip,
		log.Content,
		log.Other,
	}
}

func formatLogExportTime(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
}

func logExportType(logType int) string {
	switch logType {
	case model.LogTypeTopup:
		return "Topup"
	case model.LogTypeConsume:
		return "Consume"
	case model.LogTypeManage:
		return "Manage"
	case model.LogTypeSystem:
		return "System"
	case model.LogTypeError:
		return "Error"
	case model.LogTypeRefund:
		return "Refund"
	default:
		return "Unknown"
	}
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	stat, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	quotaNum, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quotaNum.Quota,
			"rpm":   quotaNum.Rpm,
			"tpm":   quotaNum.Tpm,
			//"token": tokenNum,
		},
	})
	return
}

func DeleteHistoryLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if targetTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target timestamp is required",
		})
		return
	}
	count, err := model.DeleteOldLog(c.Request.Context(), targetTimestamp, 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
	return
}
