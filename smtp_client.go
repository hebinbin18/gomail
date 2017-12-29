package gomail

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

const (
	CRLF = "\r\n"
	LF   = "\n"
	TAB  = "\t"
)

type auth struct {
	user     string
	password string
}

type client struct {
	auth        auth
	tls         bool
	hostPort    string
	fromAddr    string
	fromName    string
	replyAddr   string
	replyName   string
	priority    string //(1 = High, 3 = Normal, 5 = low).
	subject     string
	address     []string
	addressSend []string
	addressCC   []string
	addressBCC  []string
	attachments []map[string]string
	content     string
	mailType    string
	uniqId      string
	boundary1   string
	boundary2   string
	tryTime     int
	debug       bool
}

func NewMail() *client {
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

func (ms *client) getSendToString() string {
	send := "To: " + strings.Join(ms.addressSend, ";") + LF
	cc, bcc := "", ""
	if len(ms.addressCC) > 0 {
		cc = "Cc: " + strings.Join(ms.addressCC, ";") + LF
	} else {
		cc = ""
	}

	if len(ms.addressBCC) > 0 {
		bcc = "Bcc: " + strings.Join(ms.addressBCC, ";") + LF
	} else {
		bcc = ""
	}

	if "" == ms.fromName {
		ms.fromName = ms.fromAddr
	}
	from := fmt.Sprintf("From: =?utf-8?B?%s?= <%s>%s", base64.StdEncoding.EncodeToString([]byte(ms.fromName)), ms.fromAddr, LF)
	reply := fmt.Sprintf("Reply-to: =?utf-8?B?%s?= <%s>%s", base64.StdEncoding.EncodeToString([]byte(ms.replyName)), ms.replyAddr, LF)

	return send + cc + bcc + from + reply
}

func (ms *client) getContent() string {
	return base64.StdEncoding.EncodeToString([]byte(ms.content)) + LF
}

func (ms *client) getAttachments() string {
	if 0 == len(ms.attachments) {
		return ""
	}

	text := ""
	for _, file := range ms.attachments {
		path := file["path"]
		name := fmt.Sprintf("=?utf-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(file["name"])))
		text += "--" + ms.boundary1 + LF
		text += fmt.Sprintf(`Content-Type: application/octet-stream;%s%scharset="utf-8";%s%sname="%s"%s`, LF, TAB, LF, TAB, name, LF)
		text += fmt.Sprintf(`Content-Disposition: attachment; filename="%s"%s`, name, LF)
		text += fmt.Sprintf("Content-Transfer-Encoding: base64%s%s", LF, LF)

		fBufferB64 := ""
		if strings.HasPrefix(path, "http") {
			resp, err := http.Get(path)
			if err != nil {
				resp.Body.Close()
				panic("fetch attachments " + path + " error")
			}
			fBuffer, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				panic("fetch attachments " + path + " error2")
			}
			fBufferB64 = base64.StdEncoding.EncodeToString(fBuffer)
		} else {
			fi, err := os.Open(path)
			if err != nil {
				panic(err)
			}
			fBuffer, err := ioutil.ReadAll(fi)
			fBufferB64 = base64.StdEncoding.EncodeToString(fBuffer)
			fi.Close()
		}

		fB64LF := ""
		fLen := len(fBufferB64)
		for i := 0; i < fLen; i += 72 {
			end := i + 72
			if end > fLen {
				end = fLen
			}

			fB64LF += fBufferB64[i:end] + LF
		}
		text += fB64LF
		text += LF + LF
	}
	text += fmt.Sprintf("--%s--%s", ms.boundary1, LF)

	return text
}

// http://tools.ietf.org/html/rfc4021
func (ms *client) createHeader() string {
	h := md5.New()
	h.Write([]byte(time.Now().String()))
	ms.uniqId = hex.EncodeToString(h.Sum(nil))
	ms.boundary1 = "b1_" + ms.uniqId
	ms.boundary2 = "b2_" + ms.uniqId

	// 邮件头
	header := "Date: " + time.Now().Format(time.RFC1123Z) + LF
	header += "Return-Path: "
	if "" != ms.fromName {
		header += ms.fromName + LF
	} else {
		header += ms.fromAddr + LF
	}

	// 收件人信息
	header += ms.getSendToString()

	// 主题信息
	subject := fmt.Sprintf("Subject: =?utf-8?B?%s?=%s", base64.StdEncoding.EncodeToString([]byte(ms.subject)), LF)
	subject += fmt.Sprintf("Message-ID: <54c3aee5da3b50b47a9ee09defb8c00e@%s>%s", ms.getServerHostName(), LF)
	subject += "X-Priority: " + ms.priority + LF
	subject += "X-Mailer: GoLang (phpmailer.sourceforge.net)" + LF
	//	subject += fmt.Sprintf("Disposition-Notification-To: <%s>%s", ms.from.mailAddr, LF)
	subject += "MIME-Version: 1.0" + LF
	header += subject

	// mime type + content
	mime := ""
	if len(ms.attachments) > 0 {
		mime += "Content-Type: multipart/mixed;" + LF
		mime += fmt.Sprintf(`%sboundary="%s"%s%s%s`, TAB, ms.boundary1, LF, LF, LF)
		mime += fmt.Sprintf(`--%s%s`, ms.boundary1, LF)
		mime += fmt.Sprintf(`Content-Type: multipart/alternative;%s%sboundary="%s"%s%s`, LF, TAB, ms.boundary2, LF, LF)

		mime += "--" + ms.boundary2 + LF
		mime += `Content-Type: text/plain; charset = "utf-8"` + LF
		mime += "Content-Transfer-Encoding: base64" + LF
		mime += LF + LF
		mime += base64.StdEncoding.EncodeToString([]byte("text/html")) + LF
		mime += LF + LF
		mime += "--" + ms.boundary2 + LF
		mime += `Content-Type: text/html; charset = "utf-8"` + LF
		mime += "Content-Transfer-Encoding: base64" + LF
		mime += LF + LF
		mime += ms.getContent()
		mime += LF + LF
		mime += "--" + ms.boundary2 + "--" + LF
	} else {
		mime += fmt.Sprintf(`Content-Type: multipart/alternative;%s%sboundary="%s"%s%s`, LF, TAB, ms.boundary1, LF, LF)

		mime += "--" + ms.boundary1 + LF
		mime += `Content-Type: text/plain; charset = "utf-8"` + LF
		mime += "Content-Transfer-Encoding: base64" + LF
		mime += LF + LF
		mime += base64.StdEncoding.EncodeToString([]byte("text/html")) + LF
		mime += LF + LF
		mime += "--" + ms.boundary1 + LF
		mime += `Content-Type: text/html; charset = "utf-8"` + LF
		mime += "Content-Transfer-Encoding: base64" + LF
		mime += LF + LF
		mime += ms.getContent()
		mime += LF + LF
		mime += "--" + ms.boundary1 + "--" + LF
	}

	header += mime

	return header
}

// 真正的邮件内容: RFC 821 发送DATA以后的数据
func (ms *client) getMailContent() []byte {
	msg := ms.createHeader() + ms.getAttachments()
	if ms.debug {
		fmt.Println(msg)
	}
	return []byte(msg)
}

func (ms *client) getServerHostName() string {
	return "localhost.localdomain"
}

// 真正的邮件内容: RFC 821 发送DATA以后的数据
func (ms *client) GetMailContent() []byte {
	header := ms.createHeader()
	attachment := ms.getAttachments()
	msg := header + attachment
	if ms.debug {
		fmt.Println(header)
		fmt.Println("附件内容未打印")
	}

	return []byte(msg)
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

func (ms *client) AddAddress(address string) {
	ms.addressSend = append(ms.addressSend, address)
	ms.address = append(ms.address, address)
}

func (ms *client) AddCC(address string) {
	ms.addressCC = append(ms.addressCC, address)
	ms.address = append(ms.address, address)
}

func (ms *client) AddBCC(address string) {
	ms.addressBCC = append(ms.addressBCC, address)
	ms.address = append(ms.address, address)
}

func (ms *client) AddAttachment(path, name string) {
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

func (ms *client) SetTLS() bool {
	ms.tls = true

	return true
}

// smtp 客户端
func (ms *client) Send() error {
	defer func() {
		ms.tryTime++
		err := recover()
		if err != nil {
			stack := string(debug.Stack())
			fmt.Println(stack)
			fmt.Println(err.(error).Error())
			if ms.tryTime < 3 {
				ms.Send()
			}
		}
	}()

	cl := newMailClient(ms)

	return cl.SendMail()
}
