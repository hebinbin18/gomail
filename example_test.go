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
	m.AddAttachment("C:/Users/hebin/Desktop/123.jpg", "123.jpg")

	m.Send()
}
