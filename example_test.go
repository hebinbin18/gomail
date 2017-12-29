package gomail_test

import "github.com/hebinbin18/gomail"

func Example() {
	m := gomail.NewMail()
	m.SetHost("smtp.exmail.qq.com", 25)
	m.SetAuth("", "")

	m.SetTLS()
	m.SetFromName("")
	m.SetSubject("")
	m.AddAddress("")
	m.AddCC("")
	m.AddBCC("")

	m.SetHtmlMail()
	m.SetMailContent("this is mail content !")
	m.AddAttachment("/test.jpg", "test.jpg")

	m.Send()
}
