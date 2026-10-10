package service

import (
	"fmt"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type ServerCmdService struct{}

// List
func (is *ServerCmdService) List(page, pageSize uint) (res *model.ServerCmdList) {
	res = &model.ServerCmdList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.ServerCmd{})
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.ServerCmds)
	return
}

// Info
func (is *ServerCmdService) Info(id uint) *model.ServerCmd {
	u := &model.ServerCmd{}
	DB.Where("id = ?", id).First(u)
	return u
}

// Delete
func (is *ServerCmdService) Delete(u *model.ServerCmd) error {
	return DB.Delete(u).Error
}

// Create
func (is *ServerCmdService) Create(u *model.ServerCmd) error {
	res := DB.Create(u).Error
	return res
}

// SendCmd 发送命令
func (is *ServerCmdService) SendCmd(port int, cmd string, arg string) (string, error) {
	//组装命令
	cmd = cmd + " " + arg
	// Nextec: hbbs/hbbr em outro contêiner. O hbbs só aceita comandos vindos de 127.0.0.1, então um encaminhador (nc) ao lado
	// dele recebe na porta + deslocamento e repassa pelo loopback. NEXTEC_SERVER_CMD_HOST vazio mantém o comportamento original.
	if host := strings.TrimSpace(os.Getenv("NEXTEC_SERVER_CMD_HOST")); host != "" {
		offset := 10
		if v, e := strconv.Atoi(strings.TrimSpace(os.Getenv("NEXTEC_SERVER_CMD_PORT_OFFSET"))); e == nil && v > 0 {
			offset = v
		}
		return is.sendTo("tcp", net.JoinHostPort(host, strconv.Itoa(port+offset)), "remote", cmd)
	}
	res, err := is.SendSocketCmd("v6", port, cmd)
	if err == nil {
		return res, nil
	}
	//v6连接失败，尝试v4
	res, err = is.SendSocketCmd("v4", port, cmd)
	if err == nil {
		return res, nil
	}
	return "", err
}

// SendSocketCmd
func (is *ServerCmdService) SendSocketCmd(ty string, port int, cmd string) (string, error) {
	addr := "[::1]"
	tcp := "tcp6"
	if ty == "v4" {
		tcp = "tcp"
		addr = "127.0.0.1"
	}
	return is.sendTo(tcp, fmt.Sprintf("%s:%v", addr, port), ty, cmd)
}

// sendTo abre a conexão, envia o comando e devolve a resposta (usado pelo loopback e pelo encaminhador da Nextec).
func (is *ServerCmdService) sendTo(tcp string, address string, ty string, cmd string) (string, error) {
	conn, err := net.Dial(tcp, address)
	if err != nil {
		Logger.Debugf("%s connect to id server failed: %v", ty, err)
		return "", err
	}
	defer conn.Close()
	//发送命令
	_, err = conn.Write([]byte(cmd))
	if err != nil {
		Logger.Debugf("%s send cmd failed: %v", ty, err)
		return "", err
	}
	time.Sleep(100 * time.Millisecond)
	//读取返回
	// Nextec: comandos de gravação (aur Y, rs ...) não respondem nada; sem prazo a leitura ficava esperando para sempre
	// quando o fim da conexão não chegava (encaminhador). Sem resposta em 3 s, o comando foi enviado e o retorno é vazio.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() && n == 0 {
			return "", nil
		}
		if err.Error() != "EOF" {
			Logger.Debugf("%s read response failed: %v", ty, err)
			return "", err
		}
	}
	return string(buf[:n]), nil
}

func (is *ServerCmdService) Update(f *model.ServerCmd) error {
	return DB.Model(f).Updates(f).Error
}
