package gomail_test

import (
	"github.com/hebinbin18/gomail"
	"testing"
	"fmt"
	"time"
	"path/filepath"
)

// go test -v example_test.go -test.run Mail
func TestMail(t *testing.T) {
	m := gomail.NewClient()
	m.SetDebug()
	m.SetSSl()
	m.SetHost("smtp.163.com", 465)
	m.SetAuth("go_mail_test@163.com", "gomailtest123")

	m.SetReplyName("回复的名字")
	m.SetReplyAddr("4171947@qq.com")
	m.AddAddress("43171947@qq.com")
	m.AddCC("go_mail_test@163.com")
	m.AddBCC("43171947@qq.com")

	m.SetSubject("会议时间")
	m.SetHtmlMail()
	m.SetMailContent("今天下午 " + time.Now().Format(time.RFC3339))

	err := m.SendMail()
	fmt.Println("Test Result:", err)
}

// go test -v example_test.go -test.run MailWithAttachment
func TestMailWithAttachment(t *testing.T) {
	m := gomail.NewClient()
	m.SetDebug()
	m.SetSSl()
	m.SetHost("smtp.163.com", 465)
	m.SetAuth("go_mail_test@163.com", "gomailtest123")
	m.AddAddress("43171947@qq.com")

	m.SetSubject("会议时间")
	m.SetHtmlMail()
	m.SetMailContent("今天下午 " + time.Now().Format(time.RFC3339))

	path, _ := filepath.Abs("")
	m.AddAttachment(path+"/smtp.go", "附件-1.txt")
	m.AddAttachment(path+"/README.md", "readme.txt")

	err := m.SendMail()
	fmt.Println("Test Result:", err)
}
