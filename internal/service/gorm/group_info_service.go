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
	"go-chat/pkg/constants"
	"go-chat/pkg/enum/contact/contact_status_enum"
	"go-chat/pkg/enum/contact/contact_type_enum"
	"go-chat/pkg/enum/group_info/group_status_enum"
	"go-chat/pkg/util/random"
	"go-chat/pkg/zlog"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type groupInfoService struct {
}

var GroupInfoService = new(groupInfoService)

// SaveGroup 保存群聊
// TODO: SaveGroup 用来修改群聊消息后保存

// CreateGroup 创建群聊
func (g *groupInfoService) CreateGroup(groupReq request.CreateGroupRequest) (string, int) {
	group := model.GroupInfo{
		Uuid:      fmt.Sprintf("G%s", random.GetNowAndLenRandomString(11)),
		Name:      groupReq.Name,
		Notice:    groupReq.Notice,
		OwnerId:   groupReq.OwnerId,
		MemberCnt: 1,
		AddMode:   groupReq.AddMode,
		Avatar:    groupReq.Avatar,
		Status:    group_status_enum.NORMAL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	var members []string
	members = append(members, groupReq.OwnerId)
	var err error
	group.Members, err = json.Marshal(members)
	if err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	if res := dao.GormDB.Create(&group); res.Error != nil { // 序列化后写入mysql
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	// 添加联系人
	contact := model.UserContact{
		UserId:      groupReq.OwnerId,
		ContactId:   group.Uuid,
		ContactType: contact_type_enum.GROUP,
		Status:      group_status_enum.NORMAL,
		CreatedAt:   time.Now(),
		UpdateAt:    time.Now(),
	}

	if res := dao.GormDB.Create(&contact); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	if err := myredis.DelKeysWithPattern("contact_mygroup_list_" + groupReq.OwnerId); err != nil {
		zlog.Error(err.Error())
	}

	return "群聊创建成功", 0
}

// GetAllMembers 获取所有成员信息
//func (g *groupInfoService) GetAllMembers(groupId string) ([]string, int) {
//	var group model.GroupInfo
//	res := dao.GormDB.Preload("Members").First(&group, "uuid = ?", groupId)
//	if res.Error != nil {
//		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
//			zlog.Error("群组不存在")
//			return nil, -1
//		} else {
//			zlog.Error(res.Error.Error())
//			return nil, -1
//		}
//	} else {
//		var members []string
//		if err := json.Unmarshal(group.Members, members); err != nil {
//			zlog.Error(err.Error())
//			return nil, -1
//		}
//		return members, 0
//	}
//}

// LoadMyGroup 获取我创建的群聊
func (g *groupInfoService) LoadMyGroup(ownerId string) (string, []respond.LoadMyGroupRespond, int) {
	rspString, err := myredis.GetKeyNilIsErr("contact_mygroup_list_" + ownerId)
	if err != nil {
		if errors.Is(err, redis.Nil) { // 如果在redis中没找到
			var groupList []model.GroupInfo
			if res := dao.GormDB.Order("created_At DESC").Where("owner_id = ?", ownerId).Find(&groupList); res.Error != nil {
				zlog.Info("没有创建的群聊")
			}
			var groupListRsp []respond.LoadMyGroupRespond
			for _, group := range groupList {
				groupListRsp = append(groupListRsp, respond.LoadMyGroupRespond{
					GroupId:   group.Uuid,
					GroupName: group.Name,
					Avatar:    group.Avatar,
				})
			}
			rspString, err := json.Marshal(groupListRsp)
			if err != nil {
				zlog.Error(err.Error())
			}

			if err := myredis.SetKeyEx("contact_mygroup_list_"+ownerId, string(rspString), time.Minute*constants.REDIS_TIMEOUT); err != nil {
				zlog.Error(err.Error())
			}
			return "获取我创建的群聊成功", groupListRsp, 0
		} else {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
	}
	// 在 redis 中找到
	var groupListRsp []respond.LoadMyGroupRespond
	if err := json.Unmarshal([]byte(rspString), &groupListRsp); err != nil {
		zlog.Error(err.Error())
	}
	return "获取我创建的群聊成功", groupListRsp, 0
}

// GetGroupInfo 获取群聊详情
func (g *groupInfoService) GetGroupInfo(groupId string) (string, *respond.GetGroupInfoRespond, int) {
	rspString, err := myredis.GetKeyNilIsErr("group_info_" + groupId)
	if err != nil {
		if errors.Is(err, redis.Nil) { // 如果是 redis 中没找到
			var group model.GroupInfo
			if res := dao.GormDB.First(&group, "uuid = ?", groupId); res.Error != nil {
				// 点击获取群聊详情时，是已经在我加入的群聊中找，不会出现群聊不存在的情况 (否则是获取群聊时出错)
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, nil, -1
			}
			rsp := &respond.GetGroupInfoRespond{
				Uuid:      group.Uuid,
				Name:      group.Name,
				Notice:    group.Notice,
				Avatar:    group.Avatar,
				MemberCnt: group.MemberCnt,
				OwnerId:   group.OwnerId,
				AddMode:   group.AddMode,
				Status:    group.Status,
			}
			rspString, err := json.Marshal(rsp)
			if err != nil {
				zlog.Error(err.Error())
			}

			if err := myredis.SetKeyEx("group_info_"+groupId, string(rspString), time.Minute*constants.REDIS_TIMEOUT); err != nil {
				zlog.Error(err.Error())
			}
			return "获取群聊详情成功", rsp, 0
		} else {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
	}
	var rsp *respond.GetGroupInfoRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	return "获取群聊详情成功", rsp, 0
}

// GetGroupInfoList 获取群聊列表
// 管理员少，而且如果用户更改了，那么管理员会一直频繁删除redis，更新redis，比较麻烦，所以管理员暂时不使用redis缓存
func (g *groupInfoService) GetGroupInfoList() (string, []respond.GetGroupListRespond, int) {
	var groupList []model.GroupInfo
	if res := dao.GormDB.Unscoped().Find(&groupList); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, nil, -1
	}
	var rsp []respond.GetGroupListRespond
	for _, group := range groupList {
		rp := respond.GetGroupListRespond{
			Uuid:    group.Uuid,
			Name:    group.Name,
			OwnerId: group.OwnerId,
			Status:  group.Status,
		}
		if group.DeletedAt.Valid {
			rp.IsDeleted = true
		} else {
			rp.IsDeleted = false
		}
		rsp = append(rsp, rp)
	}
	return "获取群聊列表成功", rsp, 0
}

// LeaveGroup 退群
func (g *groupInfoService) LeaveGroup(userId string, groupId string) (string, int) {
	// 从群聊中清除该用户
	var group model.GroupInfo
	if res := dao.GormDB.First(&group, "uuid = ?", groupId); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	var members []string
	if err := json.Unmarshal(group.Members, &members); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	for i, member := range members {
		if member == userId {
			// 删除逻辑：拼接 i(userId的索引) 之前的所有元素和 i(userId的索引) 之后的所有元素
			members = append(members[:i], members[i+1:]...)
			break
		}
	}
	if data, err := json.Marshal(members); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	} else {
		group.Members = data
	}

	group.MemberCnt = len(members)
	if res := dao.GormDB.Save(&group); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}

	// 删除会话 软删除
	var deletedAt gorm.DeletedAt
	deletedAt.Time = time.Now()
	deletedAt.Valid = true
	if res := dao.GormDB.Model(&model.Session{}).Where("send_id = ? AND receive_id = ?", userId, groupId).Update("deleted_at", deletedAt); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}

	// 删除联系人
	if res := dao.GormDB.Model(&model.UserContact{}).Where("user_id = ? AND contact_id = ?", userId, groupId).Updates(map[string]interface{}{
		"deleted_at": deletedAt,
		"status":     contact_status_enum.QUIT_GROUP,
	}); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	// 删除申请记录
	if res := dao.GormDB.Model(&model.ContactApply{}).Where("contact_id = ? AND user_id = ?", groupId, userId).Update("deleted_at", deletedAt); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	// 在 redis 中删除
	if err := myredis.DelKeysWithPattern("group_info_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("groupmember_list_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("group_session_list_" + userId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("my_joined_group_list_" + userId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("session_" + userId + "_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	return "退群成功", 0
}

// DismissGroup 解散群聊
func (g *groupInfoService) DismissGroup(ownerId string, groupId string) (string, int) {
	var deletedAt gorm.DeletedAt
	deletedAt.Time = time.Now()
	deletedAt.Valid = true
	if res := dao.GormDB.Model(&model.GroupInfo{}).Where("uuid = ?", groupId).Updates(
		map[string]interface{}{
			"deleted_at": deletedAt,
			"status":     group_status_enum.DISABLE,
		}); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}

	// 批量软删除群聊相关会话记录
	var sessionList []model.Session
	if res := dao.GormDB.Model(&model.Session{}).Where("receive_id = ?", groupId).Find(&sessionList); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	for _, session := range sessionList {
		if res := dao.GormDB.Model(&session).Updates(
			map[string]interface{}{
				"deleted_at": deletedAt,
			}); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
	}

	// 批量软删除指定群聊与所有成员的联系人记录
	var userContactList []model.UserContact
	if res := dao.GormDB.Model(&model.UserContact{}).Where("contact_id = ?", groupId).Find(&userContactList); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}

	for _, userContact := range userContactList {
		if res := dao.GormDB.Model(&userContact).Update("deleted_at", deletedAt); res.Error != nil {
			zlog.Error(res.Error.Error())
		}
	}

	// 批量软删除群聊相关的加群申请记录
	var contactApplys []model.ContactApply
	if res := dao.GormDB.Model(&contactApplys).Where("contact_id = ?", groupId).Find(&contactApplys); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			zlog.Info("没有需要删除的申请记录")
			return "没有需要删除的申请记录", 0
		}
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	for _, contactApply := range contactApplys {
		if res := dao.GormDB.Model(&contactApply).Update("deleted_at", deletedAt); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
	}

	// 删除相关的缓存
	if err := myredis.DelKeysWithPattern("group_info_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("groupmember_list_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("contact_mygroup_list_" + ownerId); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPattern("group_session_list_" + ownerId); err != nil {
		zlog.Error(err.Error())
	}

	// 清除该群所有成员的已加入群列表
	var memeberIds []string
	res := dao.GormDB.Model(&model.UserContact{}).
		Where("contact_id = ? AND contact_type = ?", groupId, contact_type_enum.GROUP).
		Pluck("user_id", &memeberIds)
	if res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	} else {
		// 遍历成员ID 删除每个成员已加入群列表
		for _, memberId := range memeberIds {
			if err := myredis.DelKeysWithPattern("my_joined_group_list_" + memberId); err != nil {
				zlog.Error(err.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}
	}
	return "群聊解散成功", 0
}

// DeleteGroups 删除列表中群聊 - 管理员
func (g *groupInfoService) DeleteGroups(uuidList []string) (string, int) {
	for _, uuid := range uuidList {
		var deletedAt gorm.DeletedAt
		deletedAt.Time = time.Now()
		deletedAt.Valid = true
		if res := dao.GormDB.Model(&model.GroupInfo{}).Where("uuid = ?", uuid).Update("deleted_at", deletedAt); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}

		// 删除会话
		var sessionList []model.Session
		if res := dao.GormDB.Model(&model.Session{}).Where("receive_id = ?", uuid).Find(&sessionList); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}

		for _, session := range sessionList {
			if res := dao.GormDB.Model(&session).Where("contact_id = ?", uuid).Find(&sessionList); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}

		// 删除联系人
		var userContactList []model.UserContact
		if res := dao.GormDB.Model(&model.UserContact{}).Where("contact_id = ?", uuid).Find(&userContactList); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}

		for _, userContact := range userContactList {
			if res := dao.GormDB.Model(&userContact).Update("deleted_at", deletedAt); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}

		var contactApplys []model.ContactApply
		if res := dao.GormDB.Model(&contactApplys).Where("contact_id = ?", uuid).Find(&contactApplys); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				zlog.Info("没有需要删除的申请记录")
				return "没有需要删除的申请记录", 0
			}
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
		for _, contactApply := range contactApplys {
			if res := dao.GormDB.Model(&contactApply).Update("deleted_at", deletedAt); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}
	}
	// 批量清理 redis 缓存
	for _, uuid := range uuidList {
		// 删除群信息
		if err := myredis.DelKeysWithPattern("group_info_" + uuid); err != nil {
			zlog.Error(err.Error())
		}
		// 删除群成员列表
		if err := myredis.DelKeysWithPattern("groupmember_list_" + uuid); err != nil {
			zlog.Error(err.Error())
		}

		// 查询群成员
		var memberIds []string
		res := dao.GormDB.Model(&model.UserContact{}).
			Where("contact_id = ? AND contact_type = ?", uuid, contact_type_enum.GROUP).
			Pluck("user_id", &memberIds)
		if res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
		// 删除该群成员的已加入群列表
		for _, memberId := range memberIds {
			if err := myredis.DelKeysWithPattern("my_joined_group_list_" + memberId); err != nil {
				zlog.Error(err.Error())
				return constants.SYSTEM_ERROR, -1
			}
			if err := myredis.DelKeysWithPattern("group_session_list_" + memberId); err != nil {
				zlog.Error(err.Error())
				return constants.SYSTEM_ERROR, -1
			}
		}
	}
	return "群聊删除成功", 0
}

// CheckGroupAddMode 检查群聊加群方式
func (g *groupInfoService) CheckGroupAddMode(groupId string) (string, int8, int) {
	rspString, err := myredis.GetKeyNilIsErr("group_info_" + groupId)
	if err != nil {
		if errors.Is(err, redis.Nil) { // 如果是缓存中没有
			var group model.GroupInfo
			if res := dao.GormDB.First(&group, "uuid = ?", groupId); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1, -1
			}
			return "加群方式获取成功", group.AddMode, 0
		}
	}
	var rsp respond.GetGroupInfoRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	return "加群方式获取成功", rsp.AddMode, 0
}

// EnterGroupDirectly 直接进群
func (g *groupInfoService) EnterGroupDirectly(groupId, contactId string) (string, int) {
	// 查询群信息
	var group model.GroupInfo
	if res := dao.GormDB.First(&group, "uuid = ?", groupId); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	// 反序列化群成员列表
	var members []string
	if err := json.Unmarshal(group.Members, &members); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	// TODO: 增加重复入群校验

	// 增加新用户到成员列表并重新序列化
	members = append(members, contactId)
	if err := json.Unmarshal(group.Members, &members); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	members = append(members, contactId)
	if data, err := json.Marshal(members); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	} else {
		group.Members = data
	}
	group.MemberCnt = len(members)
	// TODO: 用 Update 替代 Save 仅更新需要的字段
	if res := dao.GormDB.Save(&group); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}

	// 添加新的联系人
	newContact := model.UserContact{
		UserId:      contactId,
		ContactId:   groupId,
		ContactType: contact_type_enum.GROUP,    // 用户
		Status:      contact_status_enum.NORMAL, // 正常
		CreatedAt:   time.Now(),
		UpdateAt:    time.Now(),
	}
	if res := dao.GormDB.Create(&newContact); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	// 清理群信息
	if err := myredis.DelKeysWithPattern("group_info_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	// 清理群成员列表
	if err := myredis.DelKeysWithPattern("groupmember_list_" + groupId); err != nil {
		zlog.Error(err.Error())
	}
	// 清理用户已加入群列表
	if err := myredis.DelKeysWithPattern("my_joined_group_list_" + contactId); err != nil {
		zlog.Error(err.Error())
	}

	return "已加入群聊", 0
}

// SetGroupStatus 设置群聊是否启用
func (g *groupInfoService) SetGroupsStatus(uuidList []string, status int8) (string, int) {
	var deletedAt gorm.DeletedAt
	deletedAt.Time = time.Now()
	deletedAt.Valid = true
	for _, uuid := range uuidList {
		if res := dao.GormDB.Model(&model.GroupInfo{}).Where("uuid = ?", uuid).Update("status", status); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
		if status == group_status_enum.DISABLE {
			var sessionList []model.Session
			if res := dao.GormDB.Model(&sessionList).Where("receive_id = ?", uuid).Find(&sessionList); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, -1
			}

			for _, session := range sessionList {
				if res := dao.GormDB.Model(&session).Update("deleted_at", deletedAt); res.Error != nil {
					zlog.Error(res.Error.Error())
					return constants.SYSTEM_ERROR, -1
				}
			}
		}
	}
	//for _, uuid := range uuidList {
	//	if err := myredis.DelKeysWithPattern("group_info_" + uuid); err != nil {
	//		zlog.Error(err.Error())
	//	}
	//}
	return "设置成功", 0
}

// UpdateGroupInfo 更新群聊消息
func (g *groupInfoService) UpdateGroupInfo(req request.UpdateGroupInfoRequest) (string, int) {
	var group model.GroupInfo
	if res := dao.GormDB.First(&group, "uuid = ?", req.Uuid); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	if req.Name != "" {
		group.Name = req.Name
	}
	if req.AddMode != -1 {
		group.AddMode = req.AddMode
	}
	if req.Notice != "" {
		group.Notice = req.Notice
	}
	if req.Avatar != "" {
		group.Avatar = req.Avatar
	}
	if res := dao.GormDB.Save(&group); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	// 修改会话
	var sessionList []model.Session
	if res := dao.GormDB.Where("receive_id = ?", req.Uuid).Find(&sessionList); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	for _, session := range sessionList {
		session.ReceiveName = group.Name
		session.Avatar = group.Avatar
		log.Println(session)
		if res := dao.GormDB.Save(&session); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
	}

	//if err := myredis.DelKeysWithPattern("group_info_" + req.Uuid); err != nil {
	//	zlog.Error(err.Error())
	//}
	//if err := myredis.SetKeyEx("contact_mygroup_list_"+ req.OwnerId, string(rspString), time.Minute*constants.REDIS_TIMEOUT); err != nil {
	//	zlog.Error(err.Error())
	//}
	return "更新成功", 0
}

// GetGroupMemberList 获取群聊成员列表
func (g *groupInfoService) GetGroupMemberList(groupId string) (string, []respond.GetGroupMemberListRespond, int) {
	rspString, err := myredis.GetKeyNilIsErr("group_memberlist_" + groupId)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			var group model.GroupInfo
			if res := dao.GormDB.First(&group, "uuid = ?", groupId); res.Error != nil {
				zlog.Error(res.Error.Error())
				return constants.SYSTEM_ERROR, nil, -1
			}
			var members []string
			if err := json.Unmarshal(group.Members, &members); err != nil {
				zlog.Error(err.Error())
				return constants.SYSTEM_ERROR, nil, -1
			}
			var rspList []respond.GetGroupMemberListRespond
			for _, member := range members {
				var user model.UserInfo
				if res := dao.GormDB.First(&user, "uuid = ?", member); res.Error != nil {
					zlog.Error(res.Error.Error())
					return constants.SYSTEM_ERROR, nil, -1
				}
				rspList = append(rspList, respond.GetGroupMemberListRespond{
					UserId:   user.Uuid,
					Nickname: user.Nickname,
					Avatar:   user.Avatar,
				})
			}
			//rspString, err := json.Marshal(rspList)
			//if err != nil {
			//	zlog.Error(err.Error())
			//}
			//if err := myredis.SetKeyEx("group_memberlist_"+groupId, string(rspString), time.Minute*constants.REDIS_TIMEOUT); err != nil {
			//	zlog.Error(err.Error())
			//}
			return "获取群聊成员列表成功", rspList, 0
		} else {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, nil, -1
		}
	}
	var rsp []respond.GetGroupMemberListRespond
	if err := json.Unmarshal([]byte(rspString), &rsp); err != nil {
		zlog.Error(err.Error())
	}
	return "获取群聊成员列表成功", rsp, 0
}

// RemoveGroupMembers 移除群聊成员
func (g *groupInfoService) RemoveGroupMembers(req request.RemoveGroupMembersRequest) (string, int) {
	var group model.GroupInfo
	if res := dao.GormDB.First(&group, "uuid = ?", req.GroupId); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	var members []string
	if err := json.Unmarshal(group.Members, &members); err != nil {
		zlog.Error(err.Error())
		return constants.SYSTEM_ERROR, -1
	}
	var deletedAt gorm.DeletedAt
	deletedAt.Time = time.Now()
	deletedAt.Valid = true
	log.Println(req.UuidList, req.OwnerId)
	for _, uuid := range req.UuidList {
		if req.OwnerId == uuid {
			return "不能移除群主", -2
		}
		// 从members中找到uuid，移除
		for i, member := range members {
			if member == uuid {
				members = append(members[:i], members[i+1:]...)
			}
		}
		group.MemberCnt -= 1
		// 删除会话
		if res := dao.GormDB.Model(&model.Session{}).Where("send_id = ? AND receive_id = ?", uuid, req.GroupId).Update("deleted_at", deletedAt); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
		// 删除联系人
		if res := dao.GormDB.Model(&model.UserContact{}).Where("user_id = ? AND contact_id = ?", uuid, req.GroupId).Update("deleted_at", deletedAt); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
		// 删除申请记录
		if res := dao.GormDB.Model(&model.ContactApply{}).Where("user_id = ? AND contact_id = ?", uuid, req.GroupId).Update("deleted_at", deletedAt); res.Error != nil {
			zlog.Error(res.Error.Error())
			return constants.SYSTEM_ERROR, -1
		}
	}
	group.Members, _ = json.Marshal(members)
	if res := dao.GormDB.Save(&group); res.Error != nil {
		zlog.Error(res.Error.Error())
		return constants.SYSTEM_ERROR, -1
	}
	//if err := myredis.DelKeysWithPattern("group_info_" + req.GroupId); err != nil {
	//	zlog.Error(err.Error())
	//}
	//if err := myredis.DelKeysWithPattern("groupmember_list_" + req.GroupId); err != nil {
	//	zlog.Error(err.Error())
	//}
	if err := myredis.DelKeysWithPrefix("group_session_list"); err != nil {
		zlog.Error(err.Error())
	}
	if err := myredis.DelKeysWithPrefix("my_joined_group_list"); err != nil {
		zlog.Error(err.Error())
	}
	return "移除群聊成员成功", 0
}
