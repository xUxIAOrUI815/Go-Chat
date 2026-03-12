package sms

import (
	"fmt"
	"strconv"
	"time"

	"go-chat/internal/config"
	"go-chat/internal/service/redis"
	"go-chat/pkg/constants"
	"go-chat/pkg/util/random"
	"go-chat/pkg/zlog"

	"github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	// util "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"

	// util "github.com/alibabacloud-go/tea-utils/service"

	"github.com/alibabacloud-go/tea/dara"
	"github.com/alibabacloud-go/tea/tea"
)

var smsClient *dysmsapi.Client

// createClient 使用 AK&SK初始化账号Client
func createClient() (result *dysmsapi.Client, err error) {
	accessKeyID := config.GetConfig().AccessKeyID
	accessKeySecret := config.GetConfig().AccessKeySecret
	if smsClient == nil {
		config := &utils.Config{
			AccessKeyId:     tea.String(accessKeyID),
			AccessKeySecret: tea.String(accessKeySecret),
		}

		config.Endpoint = tea.String("dysmsapi.aliyuncs.com")
		smsClient, err = dysmsapi.NewClient(config)
	}

	return smsClient, err
}

// 验证
func VerificationCode(telephone string) (string, int) {
	client, err := createClient()
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}

	key := "auth_code_" + telephone
	code, err := redis.GetKey(key)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}

	// 直接返回
	if code != "" {
		message := "验证码已发送，请勿重复发送"
		zlog.Info(message)
		return message, -2
	}

	// 验证码过期
	code = strconv.Itoa(random.GetRandomInt(6))
	fmt.Println(code)
	err = redis.SetKeyEx(key, code, time.Minute) // 设置过期时间  1分钟有效
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	sendSmsRequest := &dysmsapi.SendSmsRequest{
		SignName:      tea.String("阿里云短信测试"),
		TemplateCode:  tea.String("SMS_154950909"),
		TemplateParam: tea.String("{\"code\":\"" + code + "\"}"),
		PhoneNumbers:  tea.String(telephone),
	}

	runtime := &dara.RuntimeOptions{}
	rsp, err := client.SendSmsWithOptions(sendSmsRequest, runtime)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	zlog.Info(*util.ToJSONString(rsp))
	return "验证码已发送", 0
}
