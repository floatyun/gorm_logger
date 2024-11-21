package withreqid

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

/*
基本就是把gorm/logger/logger.go的logger实现复制了一下。然后把原本的换行符\n改成了\t,之后原本的各类的格式化字符串，增加了一个req_id:%s的占位符。之后各个打印日志相关的函数，实际打印的时候，从context中获取reqId，进行填充。
创建的时候，增加了一个函数参数ReqIdGetter,用于设定获取reqId的方法。
*/

type ReqIdGetter func(ctx context.Context) string

type withReqIdLogger struct {
	logger.Writer
	logger.Config
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string
	reqIdGetter                         ReqIdGetter
}

func AlwaysEmptyReqId(ctx context.Context) string {
	return ""
}

// New initialize logger
func New(writer logger.Writer, config logger.Config, getter ReqIdGetter) logger.Interface {
	var (
		infoStr      = "%s\t[info] req_id:%s "                   // file_with_line_number, req_id
		warnStr      = "%s\t[warn] req_id:%s "                   // file_with_line_number, req_id
		errStr       = "%s\t[error] req_id:%s "                  // file_with_line_number, req_id
		traceStr     = "%s\t[%.3fms] [rows:%v] %s req_id:%s "    // file_with_line_number, elapsed_time, rows, sql, req_id
		traceWarnStr = "%s %s\t[%.3fms] [rows:%v] %s req_id:%s " // file_with_line_number, 慢sql提示消息, eplased_time, rows, sql  慢sql预警
		traceErrStr  = "%s %s\t[%.3fms] [rows:%v] %s req_id:%s " // file_with_line_number, err, eplased_time, rows, sql
	)

	if config.Colorful {
		infoStr = logger.Green + "%s\t" + logger.Reset + logger.Green + "[info] req_id:%s " + logger.Reset
		warnStr = logger.BlueBold + "%s\t" + logger.Reset + logger.Magenta + "[warn] req_id:%s " + logger.Reset
		errStr = logger.Magenta + "%s\t" + logger.Reset + logger.Red + "[error] req_id:%s " + logger.Reset
		traceStr = logger.Green + "%s\t" + logger.Reset + logger.Yellow + "[%.3fms] " + logger.BlueBold + "[rows:%v]" + logger.Reset + " %s req_id:%s "
		traceWarnStr = logger.Green + "%s " + logger.Yellow + "%s\t" + logger.Reset + logger.RedBold + "[%.3fms] " + logger.Yellow + "[rows:%v]" + logger.Magenta + " %s req_id:%s " + logger.Reset
		traceErrStr = logger.RedBold + "%s " + logger.MagentaBold + "%s\t" + logger.Reset + logger.Yellow + "[%.3fms] " + logger.BlueBold + "[rows:%v]" + logger.Reset + " %s req_id:%s "
	}
	if getter == nil {
		getter = AlwaysEmptyReqId
	}

	return &withReqIdLogger{
		Writer:       writer,
		Config:       config,
		infoStr:      infoStr,
		warnStr:      warnStr,
		errStr:       errStr,
		traceStr:     traceStr,
		traceWarnStr: traceWarnStr,
		traceErrStr:  traceErrStr,
		reqIdGetter:  getter,
	}
}

// LogMode implements logger.Interface.
func (l *withReqIdLogger) LogMode(level logger.LogLevel) logger.Interface {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

// Info implements logger.Interface.
func (l withReqIdLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		l.Printf(l.infoStr+msg, append([]interface{}{utils.FileWithLineNum(), l.reqIdGetter(ctx)}, data...)...)
	}
}

// Warn implements logger.Interface.
func (l withReqIdLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		l.Printf(l.warnStr+msg, append([]interface{}{utils.FileWithLineNum(), l.reqIdGetter(ctx)}, data...)...)
	}
}

// Error implements logger.Interface.
func (l withReqIdLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		l.Printf(l.errStr+msg, append([]interface{}{utils.FileWithLineNum(), l.reqIdGetter(ctx)}, data...)...)
	}
}

// Trace implements logger.Interface.
func (l withReqIdLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	reqId := l.reqIdGetter(ctx)
	switch {
	case err != nil && l.LogLevel >= logger.Error && (!errors.Is(err, logger.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			l.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql, reqId)
		} else {
			l.Printf(l.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql, reqId)
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			l.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql, reqId)
		} else {
			l.Printf(l.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql, reqId)
		}
	case l.LogLevel == logger.Info:
		sql, rows := fc()
		if rows == -1 {
			l.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql, reqId)
		} else {
			l.Printf(l.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql, reqId)
		}
	}
}

// var _ logger.Interface = &withReqIdLogger{}
