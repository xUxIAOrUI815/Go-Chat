package logger

import (
	"go-chat/internal/config"
	"os"
	"path"
	"runtime"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger
var logPath string

func init() {
	encoderConfig := zap.NewProductionEncoderConfig()

	// 设置日志记录时间格式
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	// 日志行格式化为JSON格式
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	conf := config.GetConfig()
	logPath = conf.LogPath
	file, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 644)
	fileWriteSyncer := zapcore.AddSync(file)
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel),
		zapcore.NewCore(encoder, fileWriteSyncer, zapcore.DebugLevel),
	)
	logger = zap.New(core)
}

func getFileLogWriter() (writeSyncer zapcore.WriteSyncer) {
	// 用来切割日志文件
	lumberJackLogger := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    100,   // 单个文件最大 100 M
		MaxBackups: 60,    // 最多保留 60 个文件，超出后清理较旧的文件
		MaxAge:     1,     // 最多保留 1 天
		Compress:   false, // 是否压缩
	}

	return zapcore.AddSync(lumberJackLogger)
}

// getCallerInfoForLog 获得调用方的日志信息，包括函数名、文件名、行号
func getCallerInfoForLog() (callerFields []zap.Field) {
	pc, file, line, ok := runtime.Caller(2) //回溯两层 0: 当前调用 runtime.Caller的函数 1: 调用getCallerInfoForLog的函数 2: 调用日志函数的业务代码
	if !ok {
		return
	}
	funcName := runtime.FuncForPC(pc).Name()
	funcName = path.Base(funcName)

	callerFields = append(callerFields, zap.String("func", funcName), zap.String("file", file), zap.Int("line", line))
	return
}
func Info(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	logger.Info(message, fields...)
}

func Warn(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	logger.Warn(message, fields...)
}

func Error(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	logger.Error(message, fields...)
}

func Fatal(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	logger.Fatal(message, fields...)
}

func Debug(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	logger.Debug(message, fields...)
}
