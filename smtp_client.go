package gomail

import (
	"fmt"
	"strings"
	"errors"
)

type auth struct {
	user     string
	password string
}

type client struct {
	auth         auth
	SSL          bool
	hostPort     string
	fromAddr     string
	fromName     string
	replyAddr    string
	replyName    string
	priority     string //(1 = High, 3 = Normal, 5 = low).
	notification bool   // 已读回执
	subject      string
	address      []string // 收件人 + 抄送 + 密送
	addressSend  []string // 收件人
	addressCC    []string // 抄送
	addressBCC   []string // 密送
	attachments  []map[string]string
	content      string
	mailType     string
	tryTime      int
	debug        bool
}

func NewClient() *client {
	return &client{mailType: "text", priority: "3"}
}

// 内部方法
func (ms *client) getUserName() string {
	return ms.auth.user
}

func (ms *client) getPassword() string {
	return ms.auth.password
}

func (ms *client) getHostPort() string {
	return ms.hostPort
}

func (ms *client) getFromAddr() string {
	return ms.fromAddr
}

func (ms *client) getSendTo() []string {
	return ms.address
}

func (ms *client) getContent() []byte {
	return []byte(ms.content)
}

func (ms *client) getServerHostName() string {
	return "hebin-Pro"
}

func (ms *client) trim(s string) string {
	return strings.Trim(strings.TrimSpace(s), " \n")
}

// 外部方法
func (ms *client) SetDebug() {
	ms.debug = true

	return
}

func (ms *client) SetAuth(user, password string) {
	ms.auth.user = user
	ms.auth.password = password
	ms.fromAddr = user
}

func (ms *client) SetFromName(name string) {
	ms.fromName = name
}

func (ms *client) SetFromAddr(addr string) {
	ms.fromAddr = addr
}

func (ms *client) SetReplyAddr(addr string) {
	ms.replyAddr = addr
}

func (ms *client) SetReplyName(name string) {
	ms.replyName = name
}

func (ms *client) SetHost(host string, port int) {
	ms.hostPort = fmt.Sprintf("%s:%d", host, port)
}

func (ms *client) SetHostStr(hpStr string) {
	ms.hostPort = hpStr
}

func (ms *client) SetSubject(subject string) {
	ms.subject = subject
}

func (ms *client) ShowNotification() {
	ms.notification = true
}

func (ms *client) CloseNotification() {
	ms.notification = false
}

func (ms *client) AddAddress(address string) {
	if "" == ms.trim(address) {
		return
	}
	ms.addressSend = append(ms.addressSend, address)
	ms.address = append(ms.address, address)
}

func (ms *client) AddCC(address string) {
	if "" == ms.trim(address) {
		return
	}
	ms.addressCC = append(ms.addressCC, address)
	ms.address = append(ms.address, address)
}

func (ms *client) AddBCC(address string) {
	if "" == ms.trim(address) {
		return
	}
	ms.addressBCC = append(ms.addressBCC, address)
	ms.address = append(ms.address, address)
}

func (ms *client) AddAttachment(path, name string) {
	if "" == ms.trim(path) {
		return
	}

	file := map[string]string{"path": path, "name": name}
	ms.attachments = append(ms.attachments, file)
}

func (ms *client) SetMailContent(content string) {
	ms.content = content
}

func (ms *client) SetHtmlMail() {
	ms.SetMailType("html")
}

func (ms *client) SetMailType(t string) {
	if t != "text" && t != "html" {
		return
	}
	ms.mailType = t
}

func (ms *client) SetSSl() bool {
	ms.SSL = true

	return true
}

// SMTP 客户端
func (ms *client) SendMail() error {
	if 0 == len(ms.address) {
		return errors.New("收件人不能为空")
	}

	conn := new(goSMTPConn)
	conn.mClient = ms
	conn.debug = ms.debug
	err := conn.Send()

	return err
}
