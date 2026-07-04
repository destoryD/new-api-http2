package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

var logExportTaskSemaphore = makeLogExportTaskSemaphore()

func makeLogExportTaskSemaphore() chan struct{} {
	limit := common.GetEnvOrDefault("LOG_EXPORT_MAX_CONCURRENT", 2)
	if limit <= 0 {
		limit = 1
	}
	return make(chan struct{}, limit)
}

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
	writeLogExport(c, query.Format, true, func(handle func([]*model.Log) error) error {
		return model.ExportAllLogs(query.LogType, query.StartTimestamp, query.EndTimestamp, query.ModelName, query.Username, query.TokenName, query.Channel, query.Group, query.RequestId, query.UpstreamRequestId, 1000, handle)
	})
}

func ExportAllLogReconciliation(c *gin.Context) {
	query := parseLogExportQuery(c)
	writeLogReconciliationExport(c, query.Format, func(handle func([]*model.Log) error) error {
		return model.ExportAllLogs(model.LogTypeConsume, query.StartTimestamp, query.EndTimestamp, query.ModelName, query.Username, query.TokenName, query.Channel, query.Group, query.RequestId, query.UpstreamRequestId, 1000, handle)
	})
}

func ExportUserLogs(c *gin.Context) {
	userId := c.GetInt("id")
	query := parseLogExportQuery(c)
	writeLogExport(c, query.Format, false, func(handle func([]*model.Log) error) error {
		return model.ExportUserLogs(userId, query.LogType, query.StartTimestamp, query.EndTimestamp, query.ModelName, query.TokenName, query.Group, query.RequestId, query.UpstreamRequestId, 1000, handle)
	})
}

func ExportUserLogReconciliation(c *gin.Context) {
	userId := c.GetInt("id")
	query := parseLogExportQuery(c)
	writeLogReconciliationExport(c, query.Format, func(handle func([]*model.Log) error) error {
		return model.ExportUserLogs(userId, model.LogTypeConsume, query.StartTimestamp, query.EndTimestamp, query.ModelName, query.TokenName, query.Group, query.RequestId, query.UpstreamRequestId, 1000, handle)
	})
}

func CreateAllLogExportTask(c *gin.Context) {
	query := parseLogExportTaskQuery(c)
	createLogExportTask(c, true, query)
}

func CreateUserLogExportTask(c *gin.Context) {
	query := parseLogExportTaskQuery(c)
	createLogExportTask(c, false, query)
}

func GetAllLogExportTasks(c *gin.Context) {
	getLogExportTasks(c, true)
}

func GetUserLogExportTasks(c *gin.Context) {
	getLogExportTasks(c, false)
}

func DownloadAllLogExportTask(c *gin.Context) {
	downloadLogExportTask(c, true)
}

func DownloadUserLogExportTask(c *gin.Context) {
	downloadLogExportTask(c, false)
}

func parseLogExportTaskQuery(c *gin.Context) logExportTaskQuery {
	query := logExportTaskQuery{
		logExportQuery: parseLogExportQuery(c),
		Kind:           c.DefaultQuery("kind", "detail"),
		RawQuery:       c.Request.URL.RawQuery,
	}
	if query.Kind != "reconciliation" {
		query.Kind = "detail"
	}
	return query
}

func createLogExportTask(c *gin.Context, isAdmin bool, query logExportTaskQuery) {
	go cleanupExpiredLogExportTasks()
	userId := c.GetInt("id")
	username := c.GetString("username")
	if username == "" && userId > 0 {
		if name, err := model.GetUsernameById(userId, false); err == nil {
			username = name
		}
	}
	task := &model.LogExportTask{
		UserId:      userId,
		Username:    username,
		IsAdmin:     isAdmin,
		Kind:        query.Kind,
		Format:      query.Format,
		Status:      model.LogExportTaskStatusPending,
		Progress:    0,
		QueryParams: query.RawQuery,
	}
	if err := model.CreateLogExportTask(task); err != nil {
		common.ApiError(c, err)
		return
	}
	go runLogExportTask(task.Id, isAdmin, userId, query)
	common.ApiSuccess(c, task)
}

func getLogExportTasks(c *gin.Context, isAdmin bool) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	tasks, err := model.GetLogExportTasks(c.GetInt("id"), isAdmin, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, tasks)
}

func downloadLogExportTask(c *gin.Context, isAdmin bool) {
	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	task, err := model.GetLogExportTaskByID(taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !isAdmin && (task.UserId != c.GetInt("id") || task.IsAdmin) {
		common.ApiErrorMsg(c, "no permission to download this export task")
		return
	}
	if task.Status != model.LogExportTaskStatusSuccess || task.FilePath == "" {
		common.ApiErrorMsg(c, "export task is not ready")
		return
	}
	if _, err := os.Stat(task.FilePath); err != nil {
		common.ApiError(c, err)
		return
	}
	c.FileAttachment(task.FilePath, task.Filename)
}

type logExportTaskQuery struct {
	logExportQuery
	Kind     string
	RawQuery string
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

func writeLogExport(c *gin.Context, format string, includeRelayInfo bool, export func(func([]*model.Log) error) error) {
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

	if err := writer.Write(logExportHeaders(includeRelayInfo)); err != nil {
		common.ApiError(c, err)
		return
	}

	err := export(func(logs []*model.Log) error {
		for _, log := range logs {
			if err := writer.Write(logExportRow(log, includeRelayInfo)); err != nil {
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

func runLogExportTask(taskId int, isAdmin bool, userId int, query logExportTaskQuery) {
	logExportTaskSemaphore <- struct{}{}
	defer func() { <-logExportTaskSemaphore }()
	_ = model.UpdateLogExportTask(taskId, map[string]interface{}{
		"status":   model.LogExportTaskStatusRunning,
		"progress": 5,
	})
	filePath, filename, rows, err := generateLogExportTaskFile(taskId, isAdmin, userId, query)
	if err != nil {
		_ = model.UpdateLogExportTask(taskId, map[string]interface{}{
			"status":      model.LogExportTaskStatusFailed,
			"progress":    100,
			"error":       err.Error(),
			"finished_at": time.Now().Unix(),
		})
		if filePath != "" {
			_ = os.Remove(filePath)
		}
		common.SysError("failed to generate log export task: " + err.Error())
		return
	}
	fileSize := int64(0)
	if info, statErr := os.Stat(filePath); statErr == nil {
		fileSize = info.Size()
	}
	_ = model.UpdateLogExportTask(taskId, map[string]interface{}{
		"status":      model.LogExportTaskStatusSuccess,
		"progress":    100,
		"rows":        rows,
		"filename":    filename,
		"file_path":   filePath,
		"file_size":   fileSize,
		"finished_at": time.Now().Unix(),
	})
}

func cleanupExpiredLogExportTasks() {
	retentionHours := common.GetEnvOrDefault("LOG_EXPORT_RETENTION_HOURS", 72)
	if retentionHours <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(retentionHours) * time.Hour).Unix()
	tasks, err := model.GetExpiredLogExportTasks(cutoff, 100)
	if err != nil {
		common.SysError("failed to query expired log export tasks: " + err.Error())
		return
	}
	for _, task := range tasks {
		if task.FilePath != "" {
			_ = os.Remove(task.FilePath)
		}
		if err := model.DeleteLogExportTaskByID(task.Id); err != nil {
			common.SysError("failed to delete expired log export task: " + err.Error())
		}
	}
}

func generateLogExportTaskFile(taskId int, isAdmin bool, userId int, query logExportTaskQuery) (string, string, int, error) {
	dir := filepath.Join("data", "log_exports")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", 0, err
	}
	random, err := common.GenerateRandomCharsKey(12)
	if err != nil {
		return "", "", 0, err
	}
	prefix := "billing-logs"
	if query.Kind == "reconciliation" {
		prefix = "billing-reconciliation"
	}
	filename := fmt.Sprintf("%s-%s-%d.%s", prefix, time.Now().Format("20060102-150405"), taskId, query.Format)
	filePath := filepath.Join(dir, fmt.Sprintf("%d-%s.%s", taskId, strings.ReplaceAll(random, "/", "_"), query.Format))
	file, err := os.Create(filePath)
	if err != nil {
		return filePath, filename, 0, err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if query.Format == "txt" {
		writer.Comma = '	'
	} else if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return filePath, filename, 0, err
	}

	rows := 0
	progressRows := 0
	updateRows := func(delta int) {
		rows += delta
		progressRows += delta
		if progressRows >= 5000 {
			progressRows = 0
			_ = model.UpdateLogExportTask(taskId, map[string]interface{}{
				"rows":     rows,
				"progress": 50,
			})
		}
	}

	if query.Kind == "reconciliation" {
		err = generateLogReconciliationExport(writer, func(handle func([]*model.Log) error) error {
			if isAdmin {
				return model.ExportAllLogs(model.LogTypeConsume, query.StartTimestamp, query.EndTimestamp, query.ModelName, query.Username, query.TokenName, query.Channel, query.Group, query.RequestId, query.UpstreamRequestId, 1000, handle)
			}
			return model.ExportUserLogs(userId, model.LogTypeConsume, query.StartTimestamp, query.EndTimestamp, query.ModelName, query.TokenName, query.Group, query.RequestId, query.UpstreamRequestId, 1000, handle)
		}, updateRows)
	} else {
		err = generateLogDetailExport(writer, isAdmin, func(handle func([]*model.Log) error) error {
			if isAdmin {
				return model.ExportAllLogs(query.LogType, query.StartTimestamp, query.EndTimestamp, query.ModelName, query.Username, query.TokenName, query.Channel, query.Group, query.RequestId, query.UpstreamRequestId, 1000, handle)
			}
			return model.ExportUserLogs(userId, query.LogType, query.StartTimestamp, query.EndTimestamp, query.ModelName, query.TokenName, query.Group, query.RequestId, query.UpstreamRequestId, 1000, handle)
		}, updateRows)
	}
	writer.Flush()
	if err == nil {
		err = writer.Error()
	}
	if err != nil {
		return filePath, filename, rows, err
	}
	return filePath, filename, rows, nil
}

func generateLogDetailExport(writer *csv.Writer, includeRelayInfo bool, export func(func([]*model.Log) error) error, updateRows func(int)) error {
	if err := writer.Write(logExportHeaders(includeRelayInfo)); err != nil {
		return err
	}
	return export(func(logs []*model.Log) error {
		for _, log := range logs {
			if err := writer.Write(logExportRow(log, includeRelayInfo)); err != nil {
				return err
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
		updateRows(len(logs))
		return nil
	})
}

func generateLogReconciliationExport(writer *csv.Writer, export func(func([]*model.Log) error) error, updateRows func(int)) error {
	summaries := make(map[string]*reconciliationSummary)
	err := export(func(logs []*model.Log) error {
		for _, log := range logs {
			accumulateReconciliationSummary(summaries, log)
		}
		updateRows(len(logs))
		return nil
	})
	if err != nil {
		return err
	}
	if err := writer.Write(logReconciliationHeaders()); err != nil {
		return err
	}
	for _, summary := range sortedReconciliationSummaries(summaries) {
		if err := writer.Write(logReconciliationRow(summary)); err != nil {
			return err
		}
	}
	return nil
}

type reconciliationSummary struct {
	Group               string
	ModelName           string
	RequestCount        int
	PromptTokens        int64
	CompletionTokens    int64
	CacheReadTokens     int64
	CacheCreateTokens   int64
	CacheCreate5mTokens int64
	CacheCreate1hTokens int64
	Quota               int64
	FeeQuota            int64
	PriceComponents     map[string]struct{}
}

func accumulateReconciliationSummary(summaries map[string]*reconciliationSummary, log *model.Log) {
	other := parseLogOther(log.Other)
	group := strings.TrimSpace(log.Group)
	if group == "" {
		group = strings.TrimSpace(stringFromMap(other, "group"))
	}
	modelName := strings.TrimSpace(log.ModelName)
	key := group + "\x00" + modelName
	summary, ok := summaries[key]
	if !ok {
		summary = &reconciliationSummary{
			Group:           group,
			ModelName:       modelName,
			PriceComponents: make(map[string]struct{}),
		}
		summaries[key] = summary
	}
	summary.RequestCount++
	summary.PromptTokens += int64(log.PromptTokens)
	summary.CompletionTokens += int64(log.CompletionTokens)
	cacheCreateTokens := int64(numberFromMap(other, "cache_creation_tokens"))
	cacheCreate5mTokens := int64(numberFromMap(other, "cache_creation_tokens_5m"))
	cacheCreate1hTokens := int64(numberFromMap(other, "cache_creation_tokens_1h"))
	if cacheCreateTokens == 0 && (cacheCreate5mTokens > 0 || cacheCreate1hTokens > 0) {
		cacheCreateTokens = cacheCreate5mTokens + cacheCreate1hTokens
	}
	summary.CacheReadTokens += int64(numberFromMap(other, "cache_tokens"))
	summary.CacheCreateTokens += cacheCreateTokens
	summary.CacheCreate5mTokens += cacheCreate5mTokens
	summary.CacheCreate1hTokens += cacheCreate1hTokens
	summary.Quota += int64(log.Quota)
	feeQuota, hasFeeQuota := optionalNumberFromMap(other, "fee_quota")
	if hasFeeQuota {
		summary.FeeQuota += int64(feeQuota)
	} else {
		summary.FeeQuota += int64(log.Quota)
	}
	summary.PriceComponents[buildPriceComponent(other)] = struct{}{}
}

func writeLogReconciliationExport(c *gin.Context, format string, export func(func([]*model.Log) error) error) {
	summaries := make(map[string]*reconciliationSummary)
	err := export(func(logs []*model.Log) error {
		for _, log := range logs {
			accumulateReconciliationSummary(summaries, log)
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}

	filename := fmt.Sprintf("billing-reconciliation-%s.%s", time.Now().Format("20060102-150405"), format)
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
	if err := writer.Write(logReconciliationHeaders()); err != nil {
		common.ApiError(c, err)
		return
	}
	for _, summary := range sortedReconciliationSummaries(summaries) {
		if err := writer.Write(logReconciliationRow(summary)); err != nil {
			common.SysError("failed to export reconciliation row: " + err.Error())
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		common.SysError("failed to export reconciliation logs: " + err.Error())
	}
}

func logReconciliationHeaders() []string {
	return []string{
		"Group",
		"Model Name",
		"Requests",
		"Prompt Tokens",
		"Completion Tokens",
		"Cache Read Tokens",
		"Cache Create Tokens",
		"Cache Create 5m Tokens",
		"Cache Create 1h Tokens",
		"Total Tokens",
		"Quota",
		"Fee Quota",
		"Amount USD",
		"Fee Amount USD",
		"Price Components",
	}
}

func logReconciliationRow(summary *reconciliationSummary) []string {
	totalTokens := summary.PromptTokens + summary.CompletionTokens
	return []string{
		summary.Group,
		summary.ModelName,
		strconv.Itoa(summary.RequestCount),
		strconv.FormatInt(summary.PromptTokens, 10),
		strconv.FormatInt(summary.CompletionTokens, 10),
		strconv.FormatInt(summary.CacheReadTokens, 10),
		strconv.FormatInt(summary.CacheCreateTokens, 10),
		strconv.FormatInt(summary.CacheCreate5mTokens, 10),
		strconv.FormatInt(summary.CacheCreate1hTokens, 10),
		strconv.FormatInt(totalTokens, 10),
		strconv.FormatInt(summary.Quota, 10),
		strconv.FormatInt(summary.FeeQuota, 10),
		formatQuotaAmount(summary.Quota),
		formatQuotaAmount(summary.FeeQuota),
		strings.Join(sortedStringSet(summary.PriceComponents), " | "),
	}
}

func sortedReconciliationSummaries(summaries map[string]*reconciliationSummary) []*reconciliationSummary {
	rows := make([]*reconciliationSummary, 0, len(summaries))
	for _, summary := range summaries {
		rows = append(rows, summary)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Group != rows[j].Group {
			return rows[i].Group < rows[j].Group
		}
		return rows[i].ModelName < rows[j].ModelName
	})
	return rows
}

func sortedStringSet(set map[string]struct{}) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		if strings.TrimSpace(value) != "" {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}

func parseLogOther(other string) map[string]interface{} {
	result, _ := common.StrToMap(other)
	if result == nil {
		return map[string]interface{}{}
	}
	return result
}

func numberFromMap(data map[string]interface{}, key string) float64 {
	value, _ := optionalNumberFromMap(data, key)
	return value
}

func optionalNumberFromMap(data map[string]interface{}, key string) (float64, bool) {
	value, ok := data[key]
	if !ok || value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func stringFromMap(data map[string]interface{}, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func buildPriceComponent(other map[string]interface{}) string {
	parts := []string{}
	if mode := strings.TrimSpace(stringFromMap(other, "billing_mode")); mode != "" {
		parts = append(parts, "billing_mode="+mode)
	}
	appendNumberPart := func(label, key string) {
		if value, ok := optionalNumberFromMap(other, key); ok {
			parts = append(parts, fmt.Sprintf("%s=%g", label, value))
		}
	}
	appendNumberPart("model_ratio", "model_ratio")
	appendNumberPart("completion_ratio", "completion_ratio")
	appendNumberPart("model_price", "model_price")
	appendNumberPart("group_ratio", "group_ratio")
	appendNumberPart("user_group_ratio", "user_group_ratio")
	appendNumberPart("cache_ratio", "cache_ratio")
	appendNumberPart("cache_creation_ratio", "cache_creation_ratio")
	appendNumberPart("cache_creation_ratio_5m", "cache_creation_ratio_5m")
	appendNumberPart("cache_creation_ratio_1h", "cache_creation_ratio_1h")
	appendNumberPart("audio_ratio", "audio_ratio")
	appendNumberPart("audio_completion_ratio", "audio_completion_ratio")
	appendNumberPart("image_ratio", "image_ratio")
	appendNumberPart("web_search_price", "web_search_price")
	appendNumberPart("file_search_price", "file_search_price")
	appendNumberPart("image_generation_call_price", "image_generation_call_price")
	if tier := strings.TrimSpace(stringFromMap(other, "matched_tier")); tier != "" {
		parts = append(parts, "matched_tier="+tier)
	}
	if len(parts) == 0 {
		return "standard"
	}
	return strings.Join(parts, ";")
}

func formatQuotaAmount(quota int64) string {
	if common.QuotaPerUnit <= 0 {
		return ""
	}
	return strconv.FormatFloat(float64(quota)/float64(common.QuotaPerUnit), 'f', 6, 64)
}

func logExportHeaders(includeRelayInfo bool) []string {
	headers := []string{
		"ID",
		"Time",
		"Type",
		"Username",
		"User ID",
		"Token Name",
		"Token ID",
		"Model Name",
		"Group",
		"Quota",
		"Prompt Tokens",
		"Completion Tokens",
		"Total Tokens",
		"Duration",
		"Stream",
		"Request ID",
		"IP",
		"Content",
		"Details",
	}
	if includeRelayInfo {
		headers = append(headers[:8], append([]string{"Channel ID", "Channel"}, headers[8:]...)...)
		headers = append(headers[:18], append([]string{"Upstream Request ID"}, headers[18:]...)...)
	}
	return headers
}

func logExportRow(log *model.Log, includeRelayInfo bool) []string {
	row := []string{
		strconv.Itoa(log.Id),
		formatLogExportTime(log.CreatedAt),
		logExportType(log.Type),
		log.Username,
		strconv.Itoa(log.UserId),
		log.TokenName,
		strconv.Itoa(log.TokenId),
		log.ModelName,
		log.Group,
		strconv.Itoa(log.Quota),
		strconv.Itoa(log.PromptTokens),
		strconv.Itoa(log.CompletionTokens),
		strconv.Itoa(log.PromptTokens + log.CompletionTokens),
		strconv.Itoa(log.UseTime),
		strconv.FormatBool(log.IsStream),
		log.RequestId,
		log.Ip,
		log.Content,
		log.Other,
	}
	if includeRelayInfo {
		row = append(row[:8], append([]string{strconv.Itoa(log.ChannelId), log.ChannelName}, row[8:]...)...)
		row = append(row[:18], append([]string{log.UpstreamRequestId}, row[18:]...)...)
	}
	return row
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
