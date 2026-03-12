package random

import (
	"math"
	"math/rand"
	"strconv"
	"time"
)

func GetRandomInt(len int) int { // 生成指定长度的随机整数
	return rand.Intn(9*int(math.Pow(10, float64(len-1)))) + int(math.Pow(10, float64(len-1)))
}

func GetNowAndRandomString(len int) string { // 生成日期+指定的随机数组合字符串
	return time.Now().Format("20060102") + strconv.Itoa(GetRandomInt(len))
}
