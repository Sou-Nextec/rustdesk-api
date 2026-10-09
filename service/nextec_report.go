package service

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// Chamado na conexão e relatório mensal por cliente (Nextec).

const (
	settingTicketMode = "ticket_mode" // off | optional | required
	settingJiraBase   = "jira_base"   // ex.: https://empresa.atlassian.net/browse/

	nextecNoteBefore = int64(300) // a conexão abre até 5 min depois do clique em Conectar
	nextecNoteAfter  = int64(20)  // tolerância de relógio
	nextecReportMax  = 20000
)

var (
	nextecTicketRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{1,29}$`)
	nextecMonthRe   = regexp.MustCompile(`^(\d{4})-(0[1-9]|1[0-2])$`)
	ErrNextecTicket = errors.New("chamado inválido")
	ErrNextecMonth  = errors.New("mês inválido")
)

type NextecTicketSettings struct {
	Mode     string `json:"mode"`
	JiraBase string `json:"jira_base"`
}

func NextecTicketSettingsGet() NextecTicketSettings {
	s := NextecTicketSettings{Mode: global.NextecSettingGet(settingTicketMode), JiraBase: global.NextecSettingGet(settingJiraBase)}
	if s.Mode != "optional" && s.Mode != "required" {
		s.Mode = "off"
	}
	return s
}

func NextecTicketSettingsSet(mode, jiraBase string) error {
	if mode != "off" && mode != "optional" && mode != "required" {
		return errors.New("modo inválido")
	}
	jiraBase = strings.TrimSpace(jiraBase)
	if jiraBase != "" && !(strings.HasPrefix(jiraBase, "https://") && len(jiraBase) < 200 && !strings.ContainsAny(jiraBase, " \"'<>")) {
		return errors.New("endereço do Jira inválido (use https://...)")
	}
	if err := global.NextecSettingSet(settingTicketMode, mode); err != nil {
		return err
	}
	return global.NextecSettingSet(settingJiraBase, jiraBase)
}

// NextecNoteAdd registra o chamado de uma conexão iniciada pelo painel.
func NextecNoteAdd(u *model.User, peerID, ticket, note string) error {
	ticket = strings.TrimSpace(ticket)
	peerID = strings.ReplaceAll(strings.TrimSpace(peerID), " ", "")
	if !nextecPeerPattern.MatchString(peerID) {
		return errors.New("id inválido")
	}
	if ticket != "" && !nextecTicketRe.MatchString(ticket) {
		return ErrNextecTicket
	}
	if ticket == "" && NextecTicketSettingsGet().Mode == "required" {
		return ErrNextecTicket
	}
	n := &model.NextecConnectNote{PeerId: peerID, Ticket: strings.ToUpper(ticket), Note: truncate(strings.TrimSpace(note), 200), At: time.Now().Unix()}
	if u != nil {
		n.UserId, n.Username = u.Id, u.Username
	}
	return DB.Create(n).Error
}

// NextecReportRow é uma conexão do relatório.
type NextecReportRow struct {
	Id        uint   `json:"id"`
	StartedAt int64  `json:"started_at"`
	ClosedAt  int64  `json:"closed_at"` // 0 = não registrado
	Seconds   int64  `json:"seconds"`
	PeerId    string `json:"peer_id"`
	Machine   string `json:"machine"`
	GroupId   uint   `json:"group_id"`
	Client    string `json:"client"`
	Tech      string `json:"tech"`
	FromName  string `json:"from_name"`
	Type      int    `json:"type"`
	Ticket    string `json:"ticket"`
	Note      string `json:"note"`
}

// NextecReportMonth devolve as conexões do mês (AAAA-MM), opcionalmente de um cliente (inclui subgrupos).
func NextecReportMonth(month string, groupID uint) ([]NextecReportRow, bool, error) {
	m := nextecMonthRe.FindStringSubmatch(month)
	if m == nil {
		return nil, false, ErrNextecMonth
	}
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.Local
	}
	start, _ := time.ParseInLocation("2006-01", month, loc)
	end := start.AddDate(0, 1, 0)

	groups := map[uint]string{}
	var gl []*model.DeviceGroup
	DB.Find(&gl)
	for _, g := range gl {
		groups[g.Id] = g.Name
	}
	allowed := map[uint]bool{}
	if groupID != 0 {
		name, ok := groups[groupID]
		if !ok {
			return nil, false, errors.New("cliente não encontrado")
		}
		for id, n := range groups {
			if id == groupID || strings.HasPrefix(n, name+" / ") {
				allowed[id] = true
			}
		}
	}
	peers := map[string]*model.Peer{}
	var pl []*model.Peer
	DB.Select("id, alias, hostname, group_id").Find(&pl)
	for _, p := range pl {
		peers[p.Id] = p
	}

	var conns []*model.AuditConn
	DB.Where("created_at >= ? AND created_at < ?", start, end).Order("created_at asc").Limit(nextecReportMax + 1).Find(&conns)
	truncated := len(conns) > nextecReportMax
	if truncated {
		conns = conns[:nextecReportMax]
	}
	var notes []*model.NextecConnectNote
	DB.Where("at >= ? AND at < ?", start.Unix()-nextecNoteBefore, end.Unix()+nextecNoteAfter).Order("at asc").Find(&notes)
	byPeer := map[string][]*model.NextecConnectNote{}
	for _, n := range notes {
		byPeer[n.PeerId] = append(byPeer[n.PeerId], n)
	}

	rows := make([]NextecReportRow, 0, len(conns))
	for _, c := range conns {
		p := peers[c.PeerId]
		var gid uint
		machine := c.PeerId
		if p != nil {
			gid = p.GroupId
			if p.Alias != "" {
				machine = p.Alias
			} else if p.Hostname != "" {
				machine = p.Hostname
			}
		}
		if groupID != 0 && !allowed[gid] {
			continue
		}
		started := time.Time(c.CreatedAt).Unix()
		r := NextecReportRow{
			Id: c.Id, StartedAt: started, ClosedAt: c.CloseTime, PeerId: c.PeerId, Machine: machine, GroupId: gid,
			Client: groups[gid], Tech: c.FromName, FromName: c.FromName, Type: c.Type,
		}
		if c.CloseTime > started {
			r.Seconds = c.CloseTime - started
		}
		// chamado: a última nota da mesma máquina, até 5 min antes da conexão
		list := byPeer[c.PeerId]
		for i := len(list) - 1; i >= 0; i-- {
			if d := started - list[i].At; d >= -nextecNoteAfter && d <= nextecNoteBefore {
				r.Ticket, r.Note = list[i].Ticket, list[i].Note
				if list[i].Username != "" {
					r.Tech = list[i].Username
				}
				break
			}
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].StartedAt < rows[j].StartedAt })
	return rows, truncated, nil
}
