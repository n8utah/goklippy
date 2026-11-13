package routes

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"goklippy/backend/logic/applog"

	"github.com/gin-gonic/gin"
)

var logStore *applog.Store

// InitLogs wires the kept logs into the routes.
func InitLogs(s *applog.Store) { logStore = s }

func logFilter(c *gin.Context) applog.Filter {
	limit, _ := strconv.Atoi(c.Query("limit"))
	f := applog.Filter{MinLevel: c.DefaultQuery("level", "info"), Search: c.Query("q"), Limit: limit}
	if t, err := time.Parse(time.RFC3339Nano, c.Query("since")); err == nil {
		f.Since = t
	}
	if t, err := time.Parse(time.RFC3339Nano, c.Query("before")); err == nil {
		f.Before = t
	}
	return f
}

// ListLogs GET /logs?level=info|warn|error&q=&since=&before=&limit= : the kept logs,
// newest first; since follows live, before pages back.
func ListLogs(c *gin.Context) {
	if logStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "logs aren't kept"})
		return
	}
	entries, more := logStore.Query(logFilter(c))
	if entries == nil {
		entries = []applog.Entry{}
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries, "more": more})
}

// DownloadLogs GET /logs/download?level=&q= : the matching logs as text, oldest first,
// to attach or share.
func DownloadLogs(c *gin.Context) {
	if logStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "logs aren't kept"})
		return
	}
	f := logFilter(c)
	f.Limit = 2000
	var all []applog.Entry
	for {
		page, more := logStore.Query(f)
		all = append(all, page...)
		if !more || len(page) == 0 {
			break
		}
		f.Before = page[len(page)-1].Time
	}
	slices.Reverse(all)
	var b strings.Builder
	for _, e := range all {
		fmt.Fprintf(&b, "%s %-5s %s", e.Time.Local().Format("2006-01-02 15:04:05.000"), e.Level, e.Msg)
		keys := make([]string, 0, len(e.Attrs))
		for k := range e.Attrs {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			v := e.Attrs[k]
			if strings.ContainsAny(v, " \t\"") {
				v = strconv.Quote(v)
			}
			fmt.Fprintf(&b, " %s=%s", k, v)
		}
		b.WriteByte('\n')
	}
	name := "goklippy-logs-" + time.Now().Format("20060102-1504") + ".txt"
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(b.String()))
}

// ClearLogs DELETE /logs removes every kept log.
func ClearLogs(c *gin.Context) {
	if logStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "logs aren't kept"})
		return
	}
	if err := logStore.Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cleared": true})
}
