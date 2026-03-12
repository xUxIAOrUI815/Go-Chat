package gorm

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-chat/internal/dao"
	"go-chat/internal/dto/request"
	"go-chat/internal/dto/respond"
	"go-chat/internal/model"
	myredis "go-chat/internal/service/redis"
	"go-chat/internal/service/sms"
	"go-chat/pkg/constants"
	"go-chat/pkg/enum/user_info/user_status_enum"
	"go-chat/pkg/util/random"
	"go-chat/pkg/zlog"
	"regexp"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type userInfoService struct {
}

var UserInfoService = new(userInfoService)

// checkTelephoneValid 检验电话是否有效
func (u *userInfoService) checkTelephoneValid(telephone string) bool {
	pattern := `^1([38][0-9]|14[579]|5[^4]|16[6]|7[1-35-8]|9[189])\d{8}$`
	match, err := regexp.MatchString(pattern, telephone)
	if err != nil {
		zlog.Error(err.Error())
	}
	return match
}

// checkEmailValid 校验邮箱是否有效
func (u *userInfoService) checkEmailValid(email string) bool {
	pattern := `^[^\s@]+@[^\s@]+\.[^\s@]+$`
	match, err := regexp.MatchString(pattern, email)
	if err != nil {
		zlog.Error(err.Error())
	}
	return match
}

// checkUserIsAdminOrNot 检验用户是否为管理员
func (u *userInfoService) checkUserIsAdminOrNot(user model.UserInfo) int8 {
	return user.IsAdmin
}

// Login 登录
func (u *userInfoService) Login(loginReq request.LoginRequest) (string, *respond.LoginRespond, int) {
	password := loginReq.Password
	var user model.UserInfo // 存储从数据库查到的用户信息
	// 根据手机号查询用户 First 表示查询第一条匹配记录
	res := dao.GormDB.First(&user, "telephone = ?", loginReq.Telephone)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			message := "用户不存在，请注册"
			zlog.Error(message)
			return message, nil, -2
		}
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, -1
	}
	if user.Password != password {
		message := "密码错误"
		zlog.Error(message)
		return message, nil, -2
	}
	loginRsp := &respond.LoginRespond{
		Uuid:      user.Uuid,
		Telephone: user.Telephone,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Gender:    user.Gender,
		Birthday:  user.Birthday,
		Signature: user.Signature,
		IsAdmin:   user.IsAdmin,
		Status:    user.Status,
	}
	year, month, day := user.CreatedAt.Date()
	loginRsp.CreatedAt = fmt.Sprintf("%d-%d-%d", year, month, day)

	return "登录成功", loginRsp, 0
}

// SmsLogin 验证码登录
func (u *userInfoService) SmsLogin(req request.SmsLoginRequest) (string, *respond.LoginRespond, int) {
	var user model.UserInfo
	res := dao.GormDB.First(&user, "telephone = ?", req.Telephone)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			message := "用户不存在，请注册"
			zlog.Error(message)
			return message, nil, -2
		}
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, -1
	}

	key := "auth_code_" + req.Telephone
	code, err := myredis.GetKey(key)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, nil, -1
	}
	if code != req.SmsCode {
		message := "验证码不正确，请重试"
		zlog.Info(message)
		return message, nil, -2
	} else {
		if err := myredis.DelKeyIfExists(key); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
	}
	loginRsp := &respond.LoginRespond{
		Uuid:      user.Uuid,
		Telephone: user.Telephone,
		Nickname:  user.Nickname,
		Email:     user.Email,
		Avatar:    user.Avatar,
		Gender:    user.Gender,
		Birthday:  user.Birthday,
		Signature: user.Signature,
		IsAdmin:   user.IsAdmin,
		Status:    user.Status,
	}

	year, month, day := user.CreatedAt.Date()
	loginRsp.CreatedAt = fmt.Sprintf("%d.%d.%d", year, month, day)

	return "登录成功", loginRsp, 0
}

// SendSmsCode 发送短信验证码 - 验证码登录
func (u *userInfoService) SendSmsCode(telephone string) (string, int) {
	return sms.VerificationCode(telephone)
}

// CheckTelephoneExist 检查手机号是否存在
func (u *userInfoService) CheckTelephoneExist(telephone string) (string, int) {
	var user model.UserInfo
	if res := dao.GormDB.Where("telephone = ?", telephone).First(&user); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			zlog.Info("该手机号不存在，可以注册")
			return "", 0
		}
		zlog.Error(res.Error.Error()) // 否则错误
		return constants.SYSTEM_ERROR, -1
	}
	zlog.Info("该手机号已经存在，注册失败")
	return "该手机号已经存在，注册失败", -2
}

// Register 注册 返回(message, register_respond_string, error)
func (u *userInfoService) Register(registerReq request.RegisterRequest) (string, *respond.RegisterRespond, int) {
	key := "auth_code_" + registerReq.Telephone
	code, err := myredis.GetKey(key)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, nil, -1
	}
	if code != registerReq.SmsCode {
		message := "验证码不匹配，请重新输入"
		zlog.Info(message)
		return message, nil, -2
	} else {
		if err := myredis.DelKeyIfExists(key); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
	}
	// 前端校验手机号是否有效
	message, ret := u.CheckTelephoneExist(registerReq.Telephone)
	if ret != 0 {
		return message, nil, ret
	}
	var newUser model.UserInfo
	newUser.Uuid = "U" + random.GetNowAndLenRandomString(11)
	newUser.Telephone = registerReq.Telephone
	newUser.Password = registerReq.Password
	newUser.Nickname = registerReq.Nickname
	newUser.Avatar = "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png"
	newUser.IsAdmin = 0
	newUser.Status = user_status_enum.NORMAL

	// 手机号验证 测试阶段先不开启 👈(⌒▽⌒)👉
	//err := sms.VerificationCode(registerReq.Telephone)
	//if err != nil {
	//	zlog.Error(err.Error())
	//	return "", err
	//}

	res := dao.GormDB.Create(&newUser)
	if res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, -1
	}

	// 注册成功 chat client 建立
	// if err := chat.NewClientInit(c, newUser.Uuid); err != nil {
	// 	return "", err
	// }

	registerRsp := &respond.RegisterRespond{
		Uuid:      newUser.Uuid,
		Telephone: newUser.Telephone,
		Nickname:  newUser.Nickname,
		Email:     newUser.Email,
		Avatar:    newUser.Avatar,
		Gender:    newUser.Gender,
		Birthday:  newUser.Birthday,
		Signature: newUser.Signature,
		IsAdmin:   newUser.IsAdmin,
		Status:    newUser.Status,
	}
	year, month, day := newUser.CreatedAt.Date()
	registerRsp.CreatedAt = fmt.Sprintf("%d.%d.%d", year, month, day)

	return "注册成功", registerRsp, 0
}

// UpdateUserInfo 修改用户信息
func (u *userInfoService) UpdateUserInfo(updateReq request.UpdateUserInfoRequest) (string, int) {
	var user model.UserInfo
	if res := dao.GormDB.First(&user, "uuid = ?", updateReq.Uuid); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}

	// 逐个修改用户信息
	if updateReq.Email != "" {
		user.Email = updateReq.Email
	}
	if updateReq.Nickname != "" {
		user.Nickname = updateReq.Nickname
	}
	if updateReq.Birthday != "" {
		user.Birthday = updateReq.Birthday
	}
	if updateReq.Signature != "" {
		user.Signature = updateReq.Signature
	}
	if updateReq.Avatar != "" {
		user.Avatar = updateReq.Avatar
	}

	if res := dao.GormDB.Save(&user); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}

	return "修改成功", 0
}

// GetUserInfoList 获取用户列表	- 管理员
// 管理员从数据库中获取数据
func (u *userInfoService) GetUserInfoList(ownerId string) (string, []respond.GetUserListRespond, int) {
	// redis中没有数据，从数据库中获取
	var users []model.UserInfo
	// 获取所有用户
	if res := dao.GormDB.Unscoped().Where("uuid != ?", ownerId).Find(&users); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, -1
	}

	var rsp []respond.GetUserListRespond
	for _, user := range users {
		rp := respond.GetUserListRespond{
			Uuid:      user.Uuid,
			Telephone: user.Telephone,
			Nickname:  user.Nickname,
			Status:    user.Status,
			IsAdmin:   user.IsAdmin,
		}
		if user.DeletedAt.Valid {
			rp.IsDeleted = true
		} else {
			rp.IsDeleted = false
		}

		rsp = append(rsp, rp)
	}
	return "获取用户列表成功", rsp, 0
}

// AbleUsers 启用用户
func (u *userInfoService) AbleUsers(uuidList []string) (string, int) {
	var users []model.UserInfo
	if res := dao.GormDB.Where("uuid in (?)", uuidList).Find(&users); res.Error != nil { // 这里尝试不用反射
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	for _, user := range users {
		user.Status = user_status_enum.NORMAL
		if res := dao.GormDB.Save(&user); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
	}
	// 删除所有"contact_user_list"开头的key
	//if err := myredis.DelKeysWithPrefix("contact_user_list"); err != nil {
	//	zlog.Error(err.Error())
	//}
	return "启用用户成功", 0
}

// DisableUsers 禁用用户
// 用户是否启用禁用需要实时更新contact_user_list状态，所以redis的contact_user_list需要删除
func (u *userInfoService) DisableUsers(uuidList []string) (string, int) {
	var users []model.UserInfo
	if res := dao.GormDB.Model(model.UserInfo{}).Where("uuid in (?)", uuidList).Find(&users); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	for _, user := range users {
		// 禁用用户
		user.Status = user_status_enum.DISABLE
		if res := dao.GormDB.Save(&user); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
		// 将关联的会话软删除
		var sessionList []model.Session
		if res := dao.GormDB.Model(model.Session{}).Where("send_id = ? or receive_id = ?", user.Uuid, user.Uuid).Find(&sessionList); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
		for _, session := range sessionList {
			var deletedAt gorm.DeletedAt
			deletedAt.Time = time.Now()
			deletedAt.Valid = true
			session.DeletedAt = deletedAt
			if res := dao.GormDB.Save(&session); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}
	}
	// 删除所有"contact_user_list"开头的key
	//if err := myredis.DelKeysWithPrefix("contact_user_list"); err != nil {
	//	zlog.Error(err.Error())
	//}
	return "禁用用户成功", 0
}

// DeleteUsers 删除用户
// 用户是否启用禁用需要实时更新contact_user_list状态，所以redis的contact_user_list需要删除
func (u *userInfoService) DeleteUsers(uuidList []string) (string, int) {
	var users []model.UserInfo
	if res := dao.GormDB.Where("uuid in (?)", uuidList).Find(&users); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}

	for _, user := range users {
		user.DeletedAt.Time = time.Now()
		user.DeletedAt.Valid = true
		if res := dao.GormDB.Save(&user); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}

		// 删除会话
		var sessionList []model.Session
		if res := dao.GormDB.Where("send_id = ? or receive_id = ?", user.Uuid, user.Uuid).Find(&sessionList); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				zlog.Info(res.Error.Error())
			} else {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}
		for _, session := range sessionList {
			var deletedAt gorm.DeletedAt
			deletedAt.Time = time.Now()
			deletedAt.Valid = true
			session.DeletedAt = deletedAt
			if res := dao.GormDB.Save(&session); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}

		// 删除联系人
		var contactList []model.UserContact
		if res := dao.GormDB.Where("user_id = ? or contact_id = ?", user.Uuid, user.Uuid).Find(&contactList); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				zlog.Info(res.Error.Error())
			} else {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}
		for _, contact := range contactList {
			var deletedAt gorm.DeletedAt
			deletedAt.Time = time.Now()
			deletedAt.Valid = true
			contact.DeletedAt = deletedAt
			if res := dao.GormDB.Save(&contact); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}

		// 删除申请记录
		var applyList []model.ContactApply
		if res := dao.GormDB.Where("user_id = ? or contact_id = ?", user.Uuid, user.Uuid).Find(&applyList); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				zlog.Info(res.Error.Error())
			} else {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}
		for _, apply := range applyList {
			var deletedAt gorm.DeletedAt
			deletedAt.Time = time.Now()
			deletedAt.Valid = true
			apply.DeletedAt = deletedAt
			if res := dao.GormDB.Save(&apply); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}
	}
	// 删除所有"contact_user_list"开头的key
	//if err := myredis.DelKeysWithPrefix("contact_user_list"); err != nil {
	//	zlog.Error(err.Error())
	//}
	return "删除用户成功", 0
}

// GetUserInfo 获取用户信息
func (u *userInfoService) GetUserInfo(uuid string) (string, *respond.GetUserInfoRespond, int) {
	// redis
	zlog.Info(uuid)
	rspString, err := myredis.GetKeyNilIsErr("user_info_" + uuid)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			zlog.Info(err.Error())
			var user model.UserInfo
			if res := dao.GormDB.Where("uuid = ?", uuid).Find(&user); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, nil, -1
			}
			rsp := respond.GetUserInfoRespond{
				Uuid:      user.Uuid,
				Telephone: user.Telephone,
				Nickname:  user.Nickname,
				Avatar:    user.Avatar,
				Birthday:  user.Birthday,
				Email:     user.Email,
				Gender:    user.Gender,
				Signature: user.Signature,
				CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
				IsAdmin:   user.IsAdmin,
				Status:    user.Status,
			}
			//rspString, err := json.Marshal(rsp)
			//if err != nil {
			//	zlog.Error(err.Error())
			//}
			//if err := myredis.SetKeyEx("user_info_"+uuid, string(rspString), constants.REDIS_TIMEOUT*time.Minute); err != nil {
			//	zlog.Error(err.Error())
			//}
			return "获取用户信息成功", &rsp, 0
		} else {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
	}
	var rsp respond.GetUserInfoRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	return "获取用户信息成功", &rsp, 0
}

// SetAdmin 添加管理员
func (u *userInfoService) SetAdmin(uuidList []string, isAdmin int8) (string, int) {
	var users []model.UserInfo
	if res := dao.GormDB.Where("uuid = (?)", uuidList).Find(&users); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	for _, user := range users {
		user.IsAdmin = isAdmin
		if res := dao.GormDB.Save(&user); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
	}
	return "设置管理员成功", 0
}
