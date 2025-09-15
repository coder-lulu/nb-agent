package service

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"image/png"
	"net"
	"net/http"
	"strings"

	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/sessions"
	"github.com/pquerna/otp/totp"
	"github.com/spf13/cast"

	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/rsacrypto"
)

type UserService struct {
	t        *gotext.Locale
	conf     *koanf.Koanf
	session  *sessions.Manager
	userRepo biz.UserRepo
}

func NewUserService(t *gotext.Locale, conf *koanf.Koanf, session *sessions.Manager, user biz.UserRepo) *UserService {
	gob.Register(rsa.PrivateKey{}) // 必须注册 rsa.PrivateKey 类型否则无法反序列化 session 中的 key
	return &UserService{
		t:        t,
		conf:     conf,
		session:  session,
		userRepo: user,
	}
}

func (s *UserService) GetKey(w http.ResponseWriter, r *http.Request) {
	key, err := rsacrypto.GenerateKey()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	sess.Put("key", *key)

	pk, err := rsacrypto.PublicKeyToString(&key.PublicKey)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, pk)
}

func (s *UserService) Login(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	req, err := Bind[request.UserLogin](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	key, ok := sess.Get("key").(rsa.PrivateKey)
	if !ok {
		Error(w, http.StatusForbidden, s.t.Get("invalid key, please refresh the page"))
		return
	}

	decryptedUsername, _ := rsacrypto.DecryptData(&key, req.Username)
	decryptedPassword, _ := rsacrypto.DecryptData(&key, req.Password)
	user, err := s.userRepo.CheckPassword(string(decryptedUsername), string(decryptedPassword))
	if err != nil {
		Error(w, http.StatusForbidden, "%v", err)
		return
	}

	if user.TwoFA != "" {
		if valid := totp.Validate(req.PassCode, user.TwoFA); !valid {
			Error(w, http.StatusForbidden, s.t.Get("invalid 2FA code"))
			return
		}
	}

	// 安全登录下，将当前客户端与会话绑定
	// 安全登录只在未启用面板 HTTPS 时生效
	ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if req.SafeLogin && !s.conf.Bool("http.tls") {
		sess.Put("safe_login", true)
		sess.Put("safe_client", fmt.Sprintf("%x", sha256.Sum256([]byte(ip))))
	} else {
		sess.Forget("safe_login")
		sess.Forget("safe_client")
	}

	sess.Put("user_id", user.ID)
	sess.Forget("key")
	Success(w, nil)
}

func (s *UserService) Logout(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
	}

	sess.Forget("user_id")
	sess.Forget("key")
	sess.Forget("safe_login")
	sess.Forget("safe_client")

	Success(w, nil)
}

func (s *UserService) IsLogin(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Success(w, false)
		return
	}
	Success(w, sess.Has("user_id"))
}

func (s *UserService) IsTwoFA(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserIsTwoFA](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	twoFA, _ := s.userRepo.IsTwoFA(req.Username)
	Success(w, twoFA)
}

func (s *UserService) Info(w http.ResponseWriter, r *http.Request) {
	userID := cast.ToUint(r.Context().Value("user_id"))
	if userID == 0 {
		ErrorSystem(w)
		return
	}

	user, err := s.userRepo.Get(userID)
	if err != nil {
		ErrorSystem(w)
		return
	}

	Success(w, chix.M{
		"id":       user.ID,
		"role":     []string{"admin"},
		"username": user.Username,
		"email":    user.Email,
	})
}

func (s *UserService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	users, total, err := s.userRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": users,
	})
}

func (s *UserService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	user, err := s.userRepo.Create(req.Username, req.Password, req.Email)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, user)
}

func (s *UserService) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserUpdateUsername](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.userRepo.UpdateUsername(req.ID, req.Username); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *UserService) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserUpdatePassword](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.userRepo.UpdatePassword(req.ID, req.Password); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *UserService) UpdateEmail(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserUpdateEmail](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.userRepo.UpdateEmail(req.ID, req.Email); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *UserService) GenerateTwoFA(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	img, url, secret, err := s.userRepo.GenerateTwoFA(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	buf := new(bytes.Buffer)
	if err = png.Encode(buf, img); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"img":    base64.StdEncoding.EncodeToString(buf.Bytes()),
		"url":    url,
		"secret": secret,
	})
}

func (s *UserService) UpdateTwoFA(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserUpdateTwoFA](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.userRepo.UpdateTwoFA(req.ID, req.Code, req.Secret); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *UserService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.userRepo.Delete(req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
