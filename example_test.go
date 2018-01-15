package gomail_test

import (
	"github.com/hebinbin18/gomail"
	"testing"
	"fmt"
	"time"
)

// go test -v example_test.go -test.run Mail
func TestMail(t *testing.T) {
	m := gomail.NewClient()
	m.SetDebug()
	//m.SetSSl()
	m.SetHost("smtp.163.com", 25)
	m.SetAuth("go_mail_test@163.com", "gomailtest123")

	//m.SetReplyName("回复给我")
	//m.SetReplyAddr("4171947@qq.com")
	m.SetSubject("时间")

	m.AddAddress("hebin@zouke.com")
	m.AddCC("go_mail_test@163.com")
	m.AddCC("hebin@zouke.com")
	m.AddBCC("43171947@qq.com")

	m.SetHtmlMail()
	m.SetMailContent("今天下午见 " + time.Now().Format(time.RFC3339))
	//m.AddAttachment("/test.jpg", "test.jpg")

	err := m.SendMail()
	fmt.Println("Test Result:", err)
}
