package services

import (
	"bbs-go/common"
	"bbs-go/common/email"
	"bbs-go/common/urls"
	"bbs-go/common/validate"
	"bbs-go/model/constants"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/mlogclub/simple"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"

	"bbs-go/cache"
	"bbs-go/common/avatar"
	"bbs-go/common/uploader"

	"bbs-go/model"
	"bbs-go/repositories"

	. "github.com/ahmetb/go-linq"
)

// 邮箱验证邮件有效期（小时）
const emailVerifyExpireHour = 24

const resetTokenExpiredAfterHour = 1

var UserService = newUserService()

func newUserService() *userService {
	return &userService{}
}

type userService struct {
}

func (s *userService) Get(id int64) *model.User {
	return repositories.UserRepository.Get(simple.DB(), id)
}

func (s *userService) Take(where ...interface{}) *model.User {
	return repositories.UserRepository.Take(simple.DB(), where...)
}

func (s *userService) Find(cnd *simple.SqlCnd) []model.User {
	return repositories.UserRepository.Find(simple.DB(), cnd)
}

func (s *userService) FindOne(cnd *simple.SqlCnd) *model.User {
	return repositories.UserRepository.FindOne(simple.DB(), cnd)
}

func (s *userService) FindPageByParams(params *simple.QueryParams) (list []model.User, paging *simple.Paging) {
	return repositories.UserRepository.FindPageByParams(simple.DB(), params)
}

func (s *userService) FindPageByCnd(cnd *simple.SqlCnd) (list []model.User, paging *simple.Paging) {
	return repositories.UserRepository.FindPageByCnd(simple.DB(), cnd)
}

func (s *userService) Create(t *model.User) error {
	err := repositories.UserRepository.Create(simple.DB(), t)
	if err == nil {
		cache.UserCache.Invalidate(t.Id)
	}
	return nil
}

func (s *userService) Update(t *model.User) error {
	err := repositories.UserRepository.Update(simple.DB(), t)
	cache.UserCache.Invalidate(t.Id)
	return err
}

func (s *userService) Updates(id int64, columns map[string]interface{}) error {
	err := repositories.UserRepository.Updates(simple.DB(), id, columns)
	cache.UserCache.Invalidate(id)
	return err
}

func (s *userService) UpdateColumn(id int64, name string, value interface{}) error {
	err := repositories.UserRepository.UpdateColumn(simple.DB(), id, name, value)
	cache.UserCache.Invalidate(id)
	return err
}

func (s *userService) Delete(id int64) {
	repositories.UserRepository.Delete(simple.DB(), id)
	cache.UserCache.Invalidate(id)
}

// Scan 扫描
func (s *userService) Scan(callback func(users []model.User)) {
	var cursor int64
	for {
		list := repositories.UserRepository.Find(simple.DB(), simple.NewSqlCnd().Where("id > ?", cursor).Asc("id").Limit(100))
		if list == nil || len(list) == 0 {
			break
		}
		cursor = list[len(list)-1].Id
		callback(list)
	}
}

// Forbidden 禁言
func (s *userService) Forbidden(operatorId, userId int64, days int, reason string, r *http.Request) error {
	var forbiddenEndTime int64
	if days == -1 { // 永久禁言
		forbiddenEndTime = -1
	} else if days > 0 {
		forbiddenEndTime = simple.Timestamp(time.Now().Add(time.Hour * 24 * time.Duration(days)))
	} else {
		return errors.New("禁言时间错误")
	}
	if repositories.UserRepository.UpdateColumn(simple.DB(), userId, "forbidden_end_time", forbiddenEndTime) == nil {
		description := ""
		if simple.IsNotBlank(reason) {
			description = "禁言原因：" + reason
		}
		OperateLogService.AddOperateLog(operatorId, constants.OpTypeForbidden, constants.EntityUser, userId,
			description, r)
	}
	return nil
}

// RemoveForbidden 移除禁言
func (s *userService) RemoveForbidden(operatorId, userId int64, r *http.Request) {
	user := s.Get(userId)
	if user == nil || !user.IsForbidden() {
		return
	}
	if repositories.UserRepository.UpdateColumn(simple.DB(), userId, "forbidden_end_time", 0) == nil {
		OperateLogService.AddOperateLog(operatorId, constants.OpTypeRemoveForbidden, constants.EntityUser, userId, "", r)
	}
}

// GetByEmail 根据邮箱查找
func (s *userService) GetByEmail(email string) *model.User {
	return repositories.UserRepository.GetByEmail(simple.DB(), email)
}

// GetByUsername 根据用户名查找
func (s *userService) GetByUsername(username string) *model.User {
	return repositories.UserRepository.GetByUsername(simple.DB(), username)
}

// SignUp 注册
func (s *userService) SignUp(username, email, nickname, password, rePassword string, flag string) (*model.User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	nickname = strings.TrimSpace(nickname)

	// 验证昵称
	if len(nickname) == 0 {
		return nil, errors.New("昵称不能为空")
	}

	// 验证密码
	err := validate.IsPassword(password, rePassword)
	if err != nil {
		return nil, err
	}

	// 验证邮箱
	if len(email) > 0 {
		if err := validate.IsEmail(email); err != nil {
			return nil, err
		}
		if s.GetByEmail(email) != nil {
			return nil, errors.New("邮箱：" + email + " 已被占用")
		}
	} else {
		return nil, errors.New("请输入邮箱")
	}

	// 验证用户名
	if len(username) > 0 {
		if err := validate.IsUsername(username); err != nil {
			return nil, err
		}
		if s.isUsernameExists(username) {
			return nil, errors.New("用户名：" + username + " 已被占用")
		}
	}

	user := &model.User{
		Username:   simple.SqlNullString(username),
		Email:      simple.SqlNullString(email),
		Nickname:   nickname,
		Password:   simple.EncodePassword(password),
		Status:     constants.StatusOk,
		CreateTime: simple.NowTimestamp(),
		UpdateTime: simple.NowTimestamp(),
	}

	err = simple.Tx(simple.DB(), func(tx *gorm.DB) error {
		if err := repositories.UserRepository.Create(tx, user); err != nil {
			return err
		}

		avatarUrl, err := s.HandleAvatar(user.Id, "")
		if err != nil {
			return err
		}

		if err := repositories.UserRepository.UpdateColumn(tx, user.Id, "avatar", avatarUrl); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(flag) > 0 {
		go func(id int64, source string) {
			analyze := model.SignupAnalyze{Source: source, UserId: id}
			simple.DB().Model(&analyze).Create(&analyze)
		}(user.Id, flag)
	}
	return user, nil
}

// SignIn 登录
func (s *userService) SignIn(username, password string) (*model.User, error) {
	if len(username) == 0 {
		return nil, errors.New("用户名/邮箱不能为空")
	}
	if len(password) == 0 {
		return nil, errors.New("密码不能为空")
	}
	var user *model.User = nil
	if err := validate.IsEmail(username); err == nil { // 如果用户输入的是邮箱
		user = s.GetByEmail(username)
	} else {
		user = s.GetByUsername(username)
	}
	if user == nil || user.Status != constants.StatusOk {
		return nil, errors.New("用户不存在或被禁用")
	}
	if !simple.ValidatePassword(user.Password, password) {
		return nil, errors.New("密码错误")
	}
	return user, nil
}

// SignInByThirdAccount 第三方账号登录
func (s *userService) SignInByThirdAccount(thirdAccount *model.ThirdAccount, source string) (*model.User, *simple.CodeError) {
	user := s.Get(thirdAccount.UserId.Int64)
	if user != nil {
		if user.Status != constants.StatusOk {
			return nil, simple.NewErrorMsg("用户已被禁用")
		}
		return user, nil
	}

	var homePage string
	var description string
	if thirdAccount.ThirdType == constants.ThirdAccountTypeGithub {
		if blog := gjson.Get(thirdAccount.ExtraData, "blog"); blog.Exists() && len(blog.String()) > 0 {
			homePage = blog.String()
		} else if htmlUrl := gjson.Get(thirdAccount.ExtraData, "html_url"); htmlUrl.Exists() && len(htmlUrl.String()) > 0 {
			homePage = htmlUrl.String()
		}

		description = gjson.Get(thirdAccount.ExtraData, "bio").String()
	}

	user = &model.User{
		Username:    sql.NullString{},
		Nickname:    thirdAccount.Nickname,
		Status:      constants.StatusOk,
		HomePage:    homePage,
		Description: description,
		CreateTime:  simple.NowTimestamp(),
		UpdateTime:  simple.NowTimestamp(),
	}
	err := simple.Tx(simple.DB(), func(tx *gorm.DB) error {
		if err := repositories.UserRepository.Create(tx, user); err != nil {
			return err
		}

		if err := repositories.ThirdAccountRepository.UpdateColumn(tx, thirdAccount.Id, "user_id", user.Id); err != nil {
			return err
		}

		avatarUrl, err := s.HandleAvatar(user.Id, thirdAccount.Avatar)
		if err != nil {
			return err
		}

		if err := repositories.UserRepository.UpdateColumn(tx, user.Id, "avatar", avatarUrl); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, simple.FromError(err)
	}
	if len(source) > 0 {
		go func(id int64, source string) {
			analyze := model.SignupAnalyze{Source: source, UserId: id}
			simple.DB().Model(&analyze).Create(&analyze)
		}(user.Id, source)
	}
	cache.UserCache.Invalidate(user.Id)
	return user, nil
}

// HandleAvatar 处理头像，优先级如下：1. 如果第三方登录带有来头像；2. 生成随机默认头像
// thirdAvatar: 第三方登录带过来的头像
func (s *userService) HandleAvatar(userId int64, thirdAvatar string) (string, error) {
	if len(thirdAvatar) > 0 {
		return uploader.CopyImage(thirdAvatar)
	}

	avatarBytes, err := avatar.Generate(userId)
	if err != nil {
		return "", err
	}
	return uploader.PutImage(avatarBytes)
}

// isEmailExists 邮箱是否存在
func (s *userService) isEmailExists(email string) bool {
	if len(email) == 0 { // 如果邮箱为空，那么就认为是不存在
		return false
	}
	return s.GetByEmail(email) != nil
}

// isUsernameExists 用户名是否存在
func (s *userService) isUsernameExists(username string) bool {
	return s.GetByUsername(username) != nil
}

// SetAvatar 更新头像
func (s *userService) UpdateAvatar(userId int64, avatar string) error {
	return s.UpdateColumn(userId, "avatar", avatar)
}

// SetUsername 设置用户名
func (s *userService) SetUsername(userId int64, username string) error {
	username = strings.TrimSpace(username)
	if err := validate.IsUsername(username); err != nil {
		return err
	}

	user := s.Get(userId)
	if len(user.Username.String) > 0 {
		return errors.New("你已设置了用户名，无法重复设置。")
	}
	if s.isUsernameExists(username) {
		return errors.New("用户名：" + username + " 已被占用")
	}
	return s.UpdateColumn(userId, "username", username)
}

// SetEmail 设置密码
func (s *userService) SetEmail(userId int64, email string) error {
	email = strings.TrimSpace(email)
	if err := validate.IsEmail(email); err != nil {
		return err
	}
	if s.isEmailExists(email) {
		return errors.New("邮箱：" + email + " 已被占用")
	}
	return s.UpdateColumn(userId, "email", email)
}

// SetPassword 设置密码
func (s *userService) SetPassword(userId int64, password, rePassword string) error {
	if err := validate.IsPassword(password, rePassword); err != nil {
		return err
	}
	user := s.Get(userId)
	if len(user.Password) > 0 {
		return errors.New("你已设置了密码，如需修改请前往修改页面。")
	}
	password = simple.EncodePassword(password)
	return s.UpdateColumn(userId, "password", password)
}

// UpdatePassword 修改密码
func (s *userService) UpdatePassword(userId int64, oldPassword, password, rePassword string) error {
	if err := validate.IsPassword(password, rePassword); err != nil {
		return err
	}
	user := s.Get(userId)

	if len(user.Password) == 0 {
		return errors.New("你没设置密码，请先设置密码")
	}

	if !simple.ValidatePassword(user.Password, oldPassword) {
		return errors.New("旧密码验证失败")
	}

	return s.UpdateColumn(userId, "password", simple.EncodePassword(password))
}

// IncrTopicCount topic_count + 1
func (s *userService) IncrTopicCount(userId int64) int {
	t := repositories.UserRepository.Get(simple.DB(), userId)
	if t == nil {
		return 0
	}
	topicCount := t.TopicCount + 1
	if err := repositories.UserRepository.UpdateColumn(simple.DB(), userId, "topic_count", topicCount); err != nil {
		logrus.Error(err)
	} else {
		cache.UserCache.Invalidate(userId)
	}
	return topicCount
}

// IncrCommentCount comment_count + 1
func (s *userService) IncrCommentCount(userId int64) int {
	t := repositories.UserRepository.Get(simple.DB(), userId)
	if t == nil {
		return 0
	}
	commentCount := t.CommentCount + 1
	if err := repositories.UserRepository.UpdateColumn(simple.DB(), userId, "comment_count", commentCount); err != nil {
		logrus.Error(err)
	} else {
		cache.UserCache.Invalidate(userId)
	}
	return commentCount
}

// SyncUserCount 同步用户计数
func (s *userService) SyncUserCount() {
	s.Scan(func(users []model.User) {
		for _, user := range users {
			topicCount := repositories.TopicRepository.Count(simple.DB(), simple.NewSqlCnd().Eq("user_id", user.Id).Eq("status", constants.StatusOk))
			commentCount := repositories.CommentRepository.Count(simple.DB(), simple.NewSqlCnd().Eq("user_id", user.Id).Eq("status", constants.StatusOk))
			_ = repositories.UserRepository.UpdateColumn(simple.DB(), user.Id, "topic_count", topicCount)
			_ = repositories.UserRepository.UpdateColumn(simple.DB(), user.Id, "comment_count", commentCount)
			cache.UserCache.Invalidate(user.Id)
		}
	})
}

// SendEmailVerifyEmail 发送邮箱验证邮件
func (s *userService) SendEmailVerifyEmail(userId int64) error {
	user := s.Get(userId)
	if user == nil {
		return errors.New("用户不存在")
	}
	if user.EmailVerified {
		return errors.New("用户邮箱已验证")
	}
	if err := validate.IsEmail(user.Email.String); err != nil {
		return err
	}
	var (
		token     = simple.UUID()
		url       = urls.AbsUrl("/user/email/verify?token=" + token)
		link      = &model.ActionLink{Title: "点击这里验证邮箱>>", Url: url}
		siteTitle = cache.SysConfigCache.GetValue(constants.SysConfigSiteTitle)
		subject   = "邮箱验证 - " + siteTitle
		title     = "邮箱验证 - " + siteTitle
		content   = "该邮件用于验证你在 " + siteTitle + " 中设置邮箱的正确性，请在" + strconv.Itoa(emailVerifyExpireHour) + "小时内完成验证。验证链接：" + url
	)
	return simple.Tx(simple.DB(), func(tx *gorm.DB) error {
		if err := repositories.EmailCodeRepository.Create(tx, &model.EmailCode{
			Model:      model.Model{},
			UserId:     userId,
			Email:      user.Email.String,
			Code:       "",
			Token:      token,
			Title:      title,
			Content:    content,
			Used:       false,
			CreateTime: simple.NowTimestamp(),
		}); err != nil {
			return nil
		}
		if err := email.SendTemplateEmail(user.Email.String, subject, title, content, "", link); err != nil {
			return err
		}
		return nil
	})
}

// VerifyEmail 验证邮箱
func (s *userService) VerifyEmail(userId int64, token string) error {
	emailCode := EmailCodeService.FindOne(simple.NewSqlCnd().Eq("token", token))
	if emailCode == nil || emailCode.Used {
		return errors.New("非法请求")
	}
	if emailCode.UserId != userId {
		return errors.New("非法验证码")
	}

	if simple.TimeFromTimestamp(emailCode.CreateTime).Add(time.Hour * time.Duration(emailVerifyExpireHour)).Before(time.Now()) {
		return errors.New("验证邮件已过期")
	}
	return simple.Tx(simple.DB(), func(tx *gorm.DB) error {
		if err := repositories.UserRepository.UpdateColumn(tx, emailCode.UserId, "email_verified", true); err != nil {
			return err
		}
		cache.UserCache.Invalidate(emailCode.UserId)
		return repositories.EmailCodeRepository.UpdateColumn(tx, emailCode.Id, "used", true)
	})
}

func (s *userService) ResetPassword(email string, password string, repeatPassword string, token string) error {
	if password != repeatPassword {
		return errors.New("二次密码输入不一致")
	}
	emailCode := EmailCodeService.FindOne(simple.NewSqlCnd().Eq("token", token))
	if emailCode == nil || emailCode.Used || emailCode.Email != email {
		return errors.New("非法请求")
	}
	if simple.TimeFromTimestamp(emailCode.CreateTime).Add(time.Hour * time.Duration(resetTokenExpiredAfterHour)).Before(time.Now()) {
		return errors.New("非法请求")
	}
	return simple.Tx(simple.DB(), func(tx *gorm.DB) error {
		if err := repositories.UserRepository.UpdateColumn(tx, emailCode.UserId, "password", simple.EncodePassword(password)); err != nil {
			return err
		}
		cache.UserCache.Invalidate(emailCode.UserId)
		return repositories.EmailCodeRepository.UpdateColumn(tx, emailCode.Id, "used", true)
	})
}

// SendPasswordResetEmail 发送密码重置邮件邮件
func (s *userService) SendPasswordResetEmail(userEmail string) error {
	user := s.GetByEmail(userEmail)
	if user == nil {
		return errors.New("用户不存在")
	}
	if !user.EmailVerified {
		return errors.New("用户邮箱未验证")
	}
	if err := validate.IsEmail(user.Email.String); err != nil {
		return err
	}
	var (
		token     = simple.UUID()
		url       = urls.AbsUrl(fmt.Sprintf("/user/email/reset?token=%s&email=%s", token, user.Email.String))
		link      = &model.ActionLink{Title: "点击这里重置密码>>", Url: url}
		siteTitle = cache.SysConfigCache.GetValue(constants.SysConfigSiteTitle)
		subject   = "重置密码 - " + siteTitle
		title     = "重置密码 - " + siteTitle
		content   = "该邮件用于重置你在 " + siteTitle + " 中的密码，请在" + strconv.Itoa(resetTokenExpiredAfterHour) + "小时内完成验证。验证链接：" + url
	)
	return simple.Tx(simple.DB(), func(tx *gorm.DB) error {
		if err := repositories.EmailCodeRepository.Create(tx, &model.EmailCode{
			Model:      model.Model{},
			UserId:     user.Id,
			Email:      user.Email.String,
			Code:       "",
			Token:      token,
			Title:      title,
			Content:    content,
			Used:       false,
			CreateTime: simple.NowTimestamp(),
		}); err != nil {
			return nil
		}
		if err := email.SendTemplateEmail(user.Email.String, subject, title, content, "", link); err != nil {
			return err
		}
		return nil
	})
}

// CheckPostStatus 用于在发表内容时检查用户状态
func (s *userService) CheckPostStatus(user *model.User) *simple.CodeError {
	if user == nil {
		return simple.ErrorNotLogin
	}
	if user.Status != constants.StatusOk {
		return common.UserDisabled
	}
	if user.IsForbidden() {
		return common.ForbiddenError
	}
	observeSeconds := SysConfigService.GetInt(constants.SysConfigUserObserveSeconds)
	if user.InObservationPeriod(observeSeconds) {
		return simple.NewError(common.InObservationPeriod.Code, "账号尚在观察期，观察期时长："+strconv.Itoa(observeSeconds)+"秒，请稍后再试")
	}
	return nil
}

func (s *userService) GetRoles() *[]string {
	var roleList []string
	roles := repositories.UserRepository.GetAllRoles(simple.DB())
	for _, role := range *roles {
		if len(role) == 0 {
			continue
		}
		if strings.Contains(role, ",") {
			roleList = append(roleList, strings.Split(role, ",")...)
		} else {
			roleList = append(roleList, role)
		}
	}
	var result []string
	From(roleList).Distinct().ToSlice(&result)
	return &result
}
