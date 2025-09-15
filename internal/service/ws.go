package service

import (
	"bufio"
	"context"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"
	stdssh "golang.org/x/crypto/ssh"

	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/shell"
	"github.com/coder-lulu/nb-agent/pkg/ssh"
)

type WsService struct {
	t       *gotext.Locale
	conf    *koanf.Koanf
	sshRepo biz.SSHRepo
}

func NewWsService(t *gotext.Locale, conf *koanf.Koanf, ssh biz.SSHRepo) *WsService {
	return &WsService{
		t:       t,
		conf:    conf,
		sshRepo: ssh,
	}
}

func (s *WsService) Session(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	info, err := s.sshRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	ws, err := s.upgrade(w, r)
	if err != nil {
		ErrorSystem(w)
		return
	}
	defer func(ws *websocket.Conn) {
		_ = ws.Close()
	}(ws)

	client, err := ssh.NewSSHClient(info.Config)
	if err != nil {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, err.Error()))
		return
	}
	defer func(client *stdssh.Client) {
		_ = client.Close()
	}(client)

	turn, err := ssh.NewTurn(ws, client)
	if err != nil {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, err.Error()))
		return
	}
	defer func(turn *ssh.Turn) {
		_ = turn.Close()
	}(turn)

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = turn.Handle(ctx)
	}()
	go func() {
		defer wg.Done()
		_ = turn.Wait()
	}()

	wg.Wait()
	cancel()
}

func (s *WsService) Exec(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrade(w, r)
	if err != nil {
		ErrorSystem(w)
		return
	}
	defer func(ws *websocket.Conn) {
		_ = ws.Close()
	}(ws)

	// 第一条消息是命令
	_, cmd, err := ws.ReadMessage()
	if err != nil {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, s.t.Get("failed to read command: %v", err)))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	out, err := shell.ExecfWithPipe(ctx, string(cmd))
	if err != nil {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, s.t.Get("failed to run command: %v", err)))
		cancel()
		return
	}

	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			line := scanner.Text()
			_ = ws.WriteMessage(websocket.TextMessage, []byte(line))
		}
		if err = scanner.Err(); err != nil {
			_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, s.t.Get("failed to read command output: %v", err)))
		}
	}()

	s.readLoop(ws)
	cancel()
}

func (s *WsService) upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	upGrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}

	// debug 模式下不校验 origin，方便 vite 代理调试
	if s.conf.Bool("app.debug") {
		upGrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
	}

	return upGrader.Upgrade(w, r, nil)
}

// readLoop 阻塞直到客户端关闭连接
func (s *WsService) readLoop(c *websocket.Conn) {
	for {
		if _, _, err := c.NextReader(); err != nil {
			_ = c.Close()
			break
		}
	}
}
